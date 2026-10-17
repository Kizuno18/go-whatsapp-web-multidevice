package mediastore

import (
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestFindReusesValidatedHashOnlyWithinItsAccount(t *testing.T) {
	root := t.TempDir()
	data := []byte("cached image")
	path, err := New(root).Write("account-a", ".jpg", data, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(data)
	foundPath, found, err := Find(root, "account-a", ".jpg", digest[:], uint64(len(data)))
	if err != nil || !found || foundPath != path {
		t.Fatalf("cached attachment was not found: %s %v %v", foundPath, found, err)
	}
	if _, found, err := Find(root, "account-b", ".jpg", digest[:], uint64(len(data))); found || err != nil {
		t.Fatalf("cross-account lookup: %v %v", found, err)
	}
	if _, _, err := Find(root, "account-a", ".jpg", digest[:], 1); err == nil {
		t.Fatal("incorrect cached length accepted")
	}
}

func TestDeduplicatesConcurrentWritesWithinAccount(t *testing.T) {
	store := New(t.TempDir())
	var wg sync.WaitGroup
	paths := make(chan string, 20)
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			path, err := store.Write("account-a", ".jpg", []byte("same-image"), Limits{AccountBytes: 10, TotalBytes: 10})
			if err != nil {
				t.Error(err)
				return
			}
			paths <- path
		}()
	}
	wg.Wait()
	close(paths)
	var first string
	for path := range paths {
		if first == "" {
			first = path
		}
		if path != first {
			t.Fatalf("duplicate content received different paths: %s, %s", first, path)
		}
	}
	if store.used != 10 {
		t.Fatalf("expected one 10-byte file, got %d", store.used)
	}
	data, err := os.ReadFile(first)
	if err != nil || string(data) != "same-image" {
		t.Fatalf("invalid persisted content: %q, %v", data, err)
	}
}

func TestConcurrentWritesCannotOverrunAccountBudget(t *testing.T) {
	store := New(t.TempDir())
	var successes atomic.Int32
	var wg sync.WaitGroup
	for i := range 30 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := store.Write("account-a", ".bin", []byte{byte(i), 1}, Limits{AccountBytes: 10})
			if err == nil {
				successes.Add(1)
			} else if !errors.Is(err, ErrLimit) {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	if successes.Load() != 5 || store.used != 10 {
		t.Fatalf("budget exceeded: successes=%d, used=%d", successes.Load(), store.used)
	}
}

func TestBudgetsSurviveRestartAndPreserveOtherAccounts(t *testing.T) {
	root := t.TempDir()
	limits := Limits{AccountBytes: 5, TotalBytes: 10}
	first, err := New(root).Write("account-a", ".txt", []byte("hello"), limits)
	if err != nil {
		t.Fatal(err)
	}
	store := New(root)
	if _, err = store.Write("account-a", ".txt", []byte("more"), limits); !errors.Is(err, ErrLimit) {
		t.Fatalf("account budget not restored: %v", err)
	}
	second, err := store.Write("account-b", ".txt", []byte("hello"), limits)
	if err != nil || first == second {
		t.Fatalf("account isolation failed: %v", err)
	}
	if _, err = store.Write("account-c", ".txt", []byte("x"), limits); !errors.Is(err, ErrLimit) {
		t.Fatalf("global budget not enforced: %v", err)
	}
}

func TestLegacyFilesCountTowardTotalWithoutBeingDeleted(t *testing.T) {
	root := t.TempDir()
	legacy := filepath.Join(root, "old-file.jpg")
	if err := os.WriteFile(legacy, []byte("legacy"), 0600); err != nil {
		t.Fatal(err)
	}
	store := New(root)
	if _, err := store.Write("account", ".jpg", []byte("new"), Limits{TotalBytes: 8}); !errors.Is(err, ErrLimit) {
		t.Fatalf("legacy bytes ignored: %v", err)
	}
	if data, err := os.ReadFile(legacy); err != nil || string(data) != "legacy" {
		t.Fatal("legacy file changed")
	}
}

func TestRetentionDeletionsReleaseBudgetAfterRefresh(t *testing.T) {
	store := New(t.TempDir())
	limits := Limits{AccountBytes: 3, TotalBytes: 3}
	path, err := store.Write("account", ".txt", []byte("old"), limits)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	store.refreshed = time.Now().Add(-2 * time.Minute)
	if _, err := store.Write("account", ".txt", []byte("new"), limits); err != nil {
		t.Fatalf("deleted bytes were not reconciled: %v", err)
	}
}

func TestFilesystemReserveAndExistingFileReuse(t *testing.T) {
	store := New(t.TempDir())
	store.freeBytes = func(string) (uint64, error) { return 100, nil }
	limits := Limits{MinFreeBytes: 90}
	path, err := store.Write("account", ".txt", []byte("ten-bytes!"), limits)
	if err != nil {
		t.Fatal(err)
	}
	store.freeBytes = func(string) (uint64, error) { return 1, nil }
	if existing, err := store.Write("account", ".txt", []byte("ten-bytes!"), limits); err != nil || existing != path {
		t.Fatalf("existing media must remain available: %v", err)
	}
	if _, err := store.Write("account", ".txt", []byte("new"), limits); !errors.Is(err, ErrLimit) {
		t.Fatalf("filesystem reserve was ignored: %v", err)
	}
}

func TestRejectsSymlinkDirectoriesAndInvalidLimits(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "accounts")); err != nil {
		t.Fatal(err)
	}
	if _, err := New(root).Write("account", ".jpg", []byte("data"), Limits{}); err == nil {
		t.Fatal("symlink directory accepted")
	}
	if entries, _ := os.ReadDir(outside); len(entries) != 0 {
		t.Fatal("wrote outside media root")
	}
	store := New(t.TempDir())
	for _, limits := range []Limits{{AccountBytes: -1}, {TotalBytes: -1}, {MinFreeBytes: -1}} {
		if _, err := store.Write("account", ".jpg", []byte("data"), limits); err == nil {
			t.Fatal("negative limit accepted")
		}
	}
	if _, err := store.Write("account", "/../escape", []byte("data"), Limits{}); err == nil {
		t.Fatal("unsafe extension accepted")
	}
}

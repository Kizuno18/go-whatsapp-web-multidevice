package mediastore

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var ErrLimit = errors.New("media storage limit reached")

type Limits struct {
	AccountBytes int64
	TotalBytes   int64
	MinFreeBytes int64
}

type Store struct {
	root      string
	mu        sync.Mutex
	used      int64
	accounts  map[string]int64
	refreshed time.Time
	freeBytes func(string) (uint64, error)
}

var stores = struct {
	sync.Mutex
	items map[string]*Store
}{items: make(map[string]*Store)}

func Write(root, account, extension string, data []byte, limits Limits) (string, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	stores.Lock()
	store := stores.items[absolute]
	if store == nil {
		store = New(absolute)
		stores.items[absolute] = store
	}
	stores.Unlock()
	path, err := store.Write(account, extension, data, limits)
	if err != nil {
		return "", err
	}
	relative, err := filepath.Rel(absolute, path)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, relative), nil
}

func New(root string) *Store {
	return &Store{root: root, freeBytes: availableBytes}
}

func validExtension(extension string) bool {
	return extension == "" || (strings.HasPrefix(extension, ".") &&
		!strings.ContainsAny(extension, `/\`) && len(extension) <= 16)
}

func Find(root, account, extension string, digest []byte, size uint64) (string, bool, error) {
	if account == "" || !validExtension(extension) || len(digest) != sha256.Size {
		return "", false, nil
	}
	accountKey := fmt.Sprintf("%x", sha256.Sum256([]byte(account)))
	accountRoot := filepath.Join(root, "accounts", accountKey)
	for _, path := range []string{root, filepath.Join(root, "accounts"), accountRoot} {
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			return "", false, nil
		}
		if err != nil {
			return "", false, err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return "", false, errors.New("unsafe media cache directory")
		}
	}
	path := filepath.Join(accountRoot, fmt.Sprintf("%x", digest)+strings.ToLower(extension))
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if !info.Mode().IsRegular() || (size > 0 && uint64(info.Size()) != size) {
		return "", false, errors.New("invalid media cache entry")
	}
	return path, true, nil
}

func directory(path string) error {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		if err := os.Mkdir(path, 0755); err != nil && !os.IsExist(err) {
			return err
		}
		info, err = os.Lstat(path)
	}
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("unsafe media directory: %s", path)
	}
	return nil
}

func (s *Store) refresh() error {
	total := int64(0)
	accounts := make(map[string]int64)
	err := filepath.WalkDir(s.root, func(path string, entry fs.DirEntry, walkErr error) error {
		if os.IsNotExist(walkErr) {
			return nil
		}
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		info, err := entry.Info()
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		total += info.Size()
		relative, err := filepath.Rel(s.root, path)
		if err != nil {
			return err
		}
		parts := strings.Split(relative, string(filepath.Separator))
		if len(parts) >= 3 && parts[0] == "accounts" {
			accounts[parts[1]] += info.Size()
		}
		return nil
	})
	if err == nil {
		s.used, s.accounts, s.refreshed = total, accounts, time.Now()
	}
	return err
}

func (s *Store) Write(account, extension string, data []byte, limits Limits) (string, error) {
	if account == "" || limits.AccountBytes < 0 || limits.TotalBytes < 0 || limits.MinFreeBytes < 0 {
		return "", errors.New("invalid media storage account or limits")
	}
	if !validExtension(extension) {
		return "", errors.New("invalid media extension")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := directory(s.root); err != nil {
		return "", err
	}
	accountKey := fmt.Sprintf("%x", sha256.Sum256([]byte(account)))
	accountRoot := filepath.Join(s.root, "accounts")
	if err := directory(accountRoot); err != nil {
		return "", err
	}
	accountRoot = filepath.Join(accountRoot, accountKey)
	if err := directory(accountRoot); err != nil {
		return "", err
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(data))
	path := filepath.Join(accountRoot, digest+strings.ToLower(extension))
	info, err := os.Lstat(path)
	if err == nil {
		if !info.Mode().IsRegular() || info.Size() != int64(len(data)) {
			return "", errors.New("invalid existing media cache entry")
		}
		return path, nil
	}
	if !os.IsNotExist(err) {
		return "", err
	}

	if s.accounts == nil {
		s.accounts = make(map[string]int64)
	}
	// Retention runs outside this process; periodically reconcile its deletions.
	if (limits.AccountBytes > 0 || limits.TotalBytes > 0) &&
		(s.refreshed.IsZero() || time.Since(s.refreshed) >= time.Minute) {
		if err := s.refresh(); err != nil {
			return "", err
		}
	}
	size := int64(len(data))
	if limits.AccountBytes > 0 && size > limits.AccountBytes-s.accounts[accountKey] {
		return "", fmt.Errorf("%w: account budget", ErrLimit)
	}
	if limits.TotalBytes > 0 && size > limits.TotalBytes-s.used {
		return "", fmt.Errorf("%w: total budget", ErrLimit)
	}
	if limits.MinFreeBytes > 0 {
		free, err := s.freeBytes(s.root)
		if err != nil {
			return "", err
		}
		if uint64(size) > free || free-uint64(size) < uint64(limits.MinFreeBytes) {
			return "", fmt.Errorf("%w: filesystem reserve", ErrLimit)
		}
	}
	file, err := os.CreateTemp(accountRoot, ".media-")
	if err != nil {
		return "", err
	}
	temp := file.Name()
	defer os.Remove(temp)
	if _, err = file.Write(data); err != nil {
		_ = file.Close()
		return "", err
	}
	if err = file.Sync(); err != nil {
		_ = file.Close()
		return "", err
	}
	if err = file.Close(); err != nil {
		return "", err
	}
	if err = os.Rename(temp, path); err != nil {
		return "", err
	}
	s.used += size
	s.accounts[accountKey] += size
	return path, nil
}

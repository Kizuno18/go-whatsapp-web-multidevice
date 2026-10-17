//go:build !linux && !darwin

package mediastore

import "errors"

func availableBytes(string) (uint64, error) {
	return 0, errors.New("media filesystem reserve checks require Linux or macOS")
}

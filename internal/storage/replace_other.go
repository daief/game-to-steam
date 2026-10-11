//go:build !windows

package storage

import "os"

func replaceFile(from, to string) error { return os.Rename(from, to) }

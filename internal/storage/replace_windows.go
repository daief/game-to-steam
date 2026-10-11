//go:build windows

package storage

import "golang.org/x/sys/windows"

func replaceFile(from, to string) error {
	src, e := windows.UTF16PtrFromString(from)
	if e != nil {
		return e
	}
	dst, e := windows.UTF16PtrFromString(to)
	if e != nil {
		return e
	}
	return windows.MoveFileEx(src, dst, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH)
}

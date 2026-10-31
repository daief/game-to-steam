package system

import (
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"strconv"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// FileIcon reads an image or an embedded Windows icon without launching its file.
func FileIcon(path string) image.Image {
	path = strings.Trim(path, `"`)
	index := 0
	if i := strings.LastIndex(path, ","); i >= 0 {
		if n, err := strconv.Atoi(path[i+1:]); err == nil {
			path, index = strings.Trim(path[:i], `"`), n
		}
	}
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	cfg, _, decodeErr := image.DecodeConfig(io.LimitReader(f, 20<<20))
	var im image.Image
	if decodeErr == nil && cfg.Width > 0 && cfg.Height > 0 && cfg.Width <= 4096 && cfg.Height <= 4096 {
		_, err = f.Seek(0, 0)
		if err == nil {
			im, _, decodeErr = image.Decode(io.LimitReader(f, 20<<20))
		}
	}
	_ = f.Close()
	if im != nil && decodeErr == nil {
		return im
	}
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil
	}
	var icon uintptr
	count, _, _ := windows.NewLazySystemDLL("shell32.dll").NewProc("ExtractIconExW").Call(uintptr(unsafe.Pointer(name)), uintptr(index), uintptr(unsafe.Pointer(&icon)), 0, 1)
	if count == 0 || icon == 0 {
		return nil
	}
	user := windows.NewLazySystemDLL("user32.dll")
	gdi := windows.NewLazySystemDLL("gdi32.dll")
	defer user.NewProc("DestroyIcon").Call(icon)
	dc, _, _ := gdi.NewProc("CreateCompatibleDC").Call(0)
	if dc == 0 {
		return nil
	}
	defer gdi.NewProc("DeleteDC").Call(dc)
	// Top-down 32-bit DIB; draw onto the same opaque background as the list.
	header := struct {
		Size                   uint32
		Width, Height          int32
		Planes, Bits           uint16
		Compression, ImageSize uint32
		X, Y                   int32
		Used, Important        uint32
	}{Size: 40, Width: 32, Height: -32, Planes: 1, Bits: 32}
	var pixels unsafe.Pointer
	bm, _, _ := gdi.NewProc("CreateDIBSection").Call(dc, uintptr(unsafe.Pointer(&header)), 0, uintptr(unsafe.Pointer(&pixels)), 0, 0)
	if bm == 0 || pixels == nil {
		return nil
	}
	defer gdi.NewProc("DeleteObject").Call(bm)
	old, _, _ := gdi.NewProc("SelectObject").Call(dc, bm)
	defer gdi.NewProc("SelectObject").Call(dc, old)
	data := unsafe.Slice((*byte)(pixels), 32*32*4)
	for i := 0; i < len(data); i += 4 {
		data[i], data[i+1], data[i+2], data[i+3] = 0x35, 0x2a, 0x22, 255
	}
	ok, _, _ := user.NewProc("DrawIconEx").Call(dc, 0, 0, icon, 32, 32, 0, 0, 3)
	if ok == 0 {
		return nil
	}
	gdi.NewProc("GdiFlush").Call()
	result := image.NewRGBA(image.Rect(0, 0, 32, 32))
	for i := 0; i < len(data); i += 4 {
		result.Pix[i], result.Pix[i+1], result.Pix[i+2], result.Pix[i+3] = data[i+2], data[i+1], data[i], 255
	}
	return result
}

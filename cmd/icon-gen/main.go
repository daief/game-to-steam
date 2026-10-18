// Command icon-gen converts assets/icon.png into a multi-resolution ICO and
// an amd64 Windows resource object. Run with go generate ./assets.
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"

	"golang.org/x/image/draw"
)

func main() {
	if err := generate(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func generate() error {
	// go generate runs in assets; direct invocation runs in the project root.
	root := "."
	if _, err := os.Stat("icon.png"); err == nil {
		root = ".."
	}
	f, err := os.Open(filepath.Join(root, "assets", "icon.png"))
	if err != nil {
		return err
	}
	src, err := png.Decode(f)
	f.Close()
	if err != nil {
		return err
	}
	sizes := []int{16, 20, 24, 32, 40, 48, 64, 128, 256}
	images := make([][]byte, len(sizes))
	for i, size := range sizes {
		dst := image.NewNRGBA(image.Rect(0, 0, size, size))
		draw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Src, nil)
		var b bytes.Buffer
		if err := png.Encode(&b, dst); err != nil {
			return err
		}
		images[i] = b.Bytes()
	}
	var ico, group bytes.Buffer
	write(&ico, [3]uint16{0, 1, uint16(len(sizes))})
	write(&group, [3]uint16{0, 1, uint16(len(sizes))})
	offset := 6 + 16*len(sizes)
	for i, size := range sizes {
		header := []byte{byte(size % 256), byte(size % 256), 0, 0}
		for _, b := range []*bytes.Buffer{&ico, &group} {
			b.Write(header)
			write(b, [2]uint16{1, 32})
			write(b, uint32(len(images[i])))
		}
		write(&ico, uint32(offset))
		write(&group, uint16(i+1))
		offset += len(images[i])
	}
	for _, data := range images {
		ico.Write(data)
	}
	if err := os.WriteFile(filepath.Join(root, "assets", "icon.ico"), ico.Bytes(), 0644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, "icon_windows_amd64.syso"), resourceObject(images, group.Bytes()), 0644)
}

func write(b *bytes.Buffer, value any) { _ = binary.Write(b, binary.LittleEndian, value) }

// resourceObject emits RT_ICON (3) and RT_GROUP_ICON (14) in one COFF .rsrc
// section. Data RVAs use IMAGE_REL_AMD64_ADDR32NB relocations for Go's linker.
func resourceObject(images [][]byte, group []byte) []byte {
	types := []uint32{3, 14}
	payloads := [][][]byte{images, {group}}
	count := len(images) + 1
	dirs := 16 + 8*len(types)
	for _, entries := range payloads {
		dirs += 16 + 8*len(entries) + 24*len(entries)
	}
	dataAt := dirs + 16*count
	raw := make([]byte, dataAt)
	le := binary.LittleEndian
	dir := func(at, n int) { le.PutUint16(raw[at+14:], uint16(n)) }
	dir(0, len(types))
	next, entry := 32, dirs
	var relocations []uint32
	for i, typ := range types {
		le.PutUint32(raw[16+8*i:], typ)
		le.PutUint32(raw[20+8*i:], uint32(next)|0x80000000)
		idDir := next
		entries := payloads[i]
		dir(idDir, len(entries))
		next += 16 + 8*len(entries)
		for j, data := range entries {
			le.PutUint32(raw[idDir+16+8*j:], uint32(j+1))
			le.PutUint32(raw[idDir+20+8*j:], uint32(next)|0x80000000)
			dir(next, 1)
			le.PutUint32(raw[next+16:], 0) // language neutral
			le.PutUint32(raw[next+20:], uint32(entry))
			next += 24
			for len(raw)%4 != 0 {
				raw = append(raw, 0)
			}
			le.PutUint32(raw[entry:], uint32(len(raw)))
			le.PutUint32(raw[entry+4:], uint32(len(data)))
			relocations = append(relocations, uint32(entry))
			entry += 16
			raw = append(raw, data...)
		}
	}
	for len(raw)%4 != 0 {
		raw = append(raw, 0)
	}
	relocAt := 60 + len(raw)
	symbolAt := relocAt + 10*len(relocations)
	var out bytes.Buffer
	write(&out, uint16(0x8664))
	write(&out, uint16(1))
	write(&out, uint32(0))
	write(&out, uint32(symbolAt))
	write(&out, uint32(1))
	write(&out, uint16(0))
	write(&out, uint16(0))
	out.Write([]byte{'.', 'r', 's', 'r', 'c', 0, 0, 0})
	write(&out, [2]uint32{0, 0})
	write(&out, uint32(len(raw)))
	write(&out, uint32(60))
	write(&out, uint32(relocAt))
	write(&out, uint32(0))
	write(&out, uint16(len(relocations)))
	write(&out, uint16(0))
	write(&out, uint32(0x40000040))
	out.Write(raw)
	for _, at := range relocations {
		write(&out, at)
		write(&out, uint32(0))
		write(&out, uint16(3))
	}
	out.Write([]byte{'.', 'r', 's', 'r', 'c', 0, 0, 0})
	write(&out, uint32(0))
	write(&out, uint16(1))
	write(&out, uint16(0))
	out.Write([]byte{3, 0})
	write(&out, uint32(4))
	return out.Bytes()
}

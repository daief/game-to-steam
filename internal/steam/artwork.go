package steam

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"game-to-steam/internal/model"
)

func ArtBase(id uint32, kind model.ArtworkKind) string {
	base := strconv.FormatUint(uint64(id), 10)
	switch kind {
	case model.Portrait:
		return base + "p"
	case model.Hero:
		return base + "_hero"
	case model.Logo:
		return base + "_logo"
	}
	return base
}
func ArtFiles(dir string, id uint32, kind model.ArtworkKind) ([]string, error) {
	entries, e := os.ReadDir(dir)
	if os.IsNotExist(e) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	base := ArtBase(id, kind)
	var paths []string
	for _, item := range entries {
		if item.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(item.Name()))
		if strings.TrimSuffix(item.Name(), filepath.Ext(item.Name())) == base && (ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".webp") {
			paths = append(paths, filepath.Join(dir, item.Name()))
		}
	}
	return paths, nil
}
func AllArtFiles(dir string, id uint32) ([]string, error) {
	var files []string
	for _, kind := range append(append([]model.ArtworkKind{}, model.ArtworkKinds...), model.ArtworkKind("icon"), model.ArtworkKind("bigpicture")) {
		var paths []string
		var e error
		if kind == "icon" || kind == "bigpicture" {
			entries, err := os.ReadDir(dir)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				return nil, err
			}
			base := fmt.Sprintf("%d_%s", id, kind)
			for _, entry := range entries {
				ext := strings.ToLower(filepath.Ext(entry.Name()))
				name := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
				matches := name == base || (kind == "icon" && name == fmt.Sprintf("%d-icon", id))
				if !entry.IsDir() && matches && (ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".ico" || ext == ".webp") {
					paths = append(paths, filepath.Join(dir, entry.Name()))
				}
			}
		} else {
			paths, e = ArtFiles(dir, id, kind)
		}
		if e != nil {
			return nil, e
		}
		files = append(files, paths...)
	}
	return files, nil
}

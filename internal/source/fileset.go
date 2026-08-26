package source

import (
	"os"
	"path/filepath"
	"strings"
)

// File represents a single source file.
type File struct {
	Path    string
	Content []byte
}

// FileSet holds all source files for a check run.
type FileSet struct {
	Files []File
}

// LoadDir recursively loads all .rs files from dir into a FileSet.
func LoadDir(dir string) (*FileSet, error) {
	fs := &FileSet{}
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".rs") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		fs.Files = append(fs.Files, File{Path: path, Content: data})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return fs, nil
}

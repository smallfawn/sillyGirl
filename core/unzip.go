package core

import (
	"archive/zip"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func unzip(filename string, perm fs.FileMode) error {
	zipFile, err := zip.OpenReader(filename)
	if err != nil {
		return err
	}
	defer zipFile.Close()
	top := ""
	baseDir := filepath.Clean(filepath.Dir(filename))
	for i := range zipFile.File {
		file := zipFile.File[i]
		if top == "" {
			top = strings.Split(file.Name, "/")[0]
		}
		if strings.HasPrefix(file.Name, "__MACOSX/") {
			continue
		}
		path := filepath.Clean(filepath.Join(baseDir, file.Name))
		rel, err := filepath.Rel(baseDir, path)
		if err != nil || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
			continue
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(path, perm); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), perm); err != nil {
			return err
		}
		if err := func() error {
			zipFile, err := file.Open()
			if err != nil {
				return err
			}
			defer zipFile.Close()

			localFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
			if err != nil {
				return err
			}
			defer localFile.Close()
			_, err = io.Copy(localFile, zipFile)
			return err
		}(); err != nil {
			return err
		}
	}
	return nil
}

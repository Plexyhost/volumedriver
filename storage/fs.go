package storage

import (
	"io"
	"os"
	"strings"
)

type fsStorage struct {
	root   string
	suffix string
}

func NewFSStorage(root string) StorageProvider {
	os.MkdirAll(root, os.ModeDir)

	if !strings.HasSuffix(root, "/") {
		root += "/"
	}

	return fsStorage{
		root:   root,
		suffix: ".tar.gz",
	}
}

func (fs fsStorage) Store(id string, src io.Reader) error {
	f, err := os.Create(fs.root + id + fs.suffix)
	if err != nil {
		return err
	}
	_, err = f.ReadFrom(src)
	return err
}

func (fs fsStorage) Retrieve(id string, dst io.Writer) error {
	f, err := os.Open(fs.root + id + fs.suffix)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteTo(dst)
	return err
}

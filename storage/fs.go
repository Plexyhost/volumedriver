package storage

import (
	"fmt"
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
	fmt.Println("store: storing...")
	f, err := os.Create(fs.root + id + fs.suffix)
	if err != nil {
		return err
	}
	s, _ := f.Stat()
	fmt.Printf("s.Name(): %v\n", s.Name())
	var n int64
	fmt.Println("store: file is reading from src")
	n, err = f.ReadFrom(src)
	fmt.Println("stored", n, "bytes")
	return err
}

func (fs fsStorage) Retrieve(id string, dst io.Writer) error {
	f, err := os.Open(fs.root + id + fs.suffix)
	if err != nil {
		return err
	}

	_, err = f.WriteTo(dst)
	return err
}

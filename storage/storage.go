package storage

import "io"

type StorageProvider interface {
	Store(id string, src io.Reader) error
	Retrieve(id string, dst io.Writer) error
}

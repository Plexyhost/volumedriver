package driver

import (
	"bytes"
	"errors"
	"fmt"
	"os"

	"github.com/plexyhost/volume-driver/cmp"
	"github.com/plexyhost/volume-driver/storage"
)

func (d *plexVolumeDriver) saveToStore(vol *volumeInfo) error {
	var buf bytes.Buffer

	err := cmp.Compress(vol.Mountpoint, &buf)
	if err != nil {
		return err
	}

	fmt.Println("compressed all")

	return d.store.Store(vol.ServerID, &buf)
}

func (d *plexVolumeDriver) loadFromStore(vol *volumeInfo) error {
	var buf bytes.Buffer

	fmt.Println("loading from store")

	err := d.store.Retrieve(vol.ServerID, &buf)
	fmt.Printf("err: %v\n", err)
	if errors.Is(err, os.ErrNotExist) || err == storage.ErrCacheHit {
		return nil
	}

	if err != nil {
		return err
	}

	return cmp.Decompress(&buf, vol.Mountpoint)
}

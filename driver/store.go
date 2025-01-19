package driver

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/plexyhost/volume-driver/pkg/compression"
	"github.com/plexyhost/volume-driver/storage"

	"github.com/charmbracelet/log"
)

func (d *PlexVolumeDriver) saveToStore(vol *volumeInfo) error {
	buf := bytes.NewBuffer(make([]byte, 0, 1024*1024)) // Pre-allocate 1MB
	start := time.Now()

	err := compression.Compress(vol.Mountpoint, buf)
	if err != nil {
		log.Errorf("Error while compressing %s: %s", vol.ServerID, err)
		return err
	}
	log.Infof("Compressed %s in %s", vol.ServerID, time.Since(start))

	start = time.Now()
	err = d.store.Store(vol.ServerID, buf)
	if err != nil {
		log.Errorf("Error while storing %s: %s", vol.ServerID, err)
		return err
	}
	log.Infof("Stored %s in %s", vol.ServerID, time.Since(start))
	return nil
}

func (d *PlexVolumeDriver) loadFromStore(vol *volumeInfo) error {
	var buf bytes.Buffer

	fmt.Println("loading from store")

	err := d.store.Retrieve(vol.ServerID, &buf)
	fmt.Printf("err: %v\n", err)
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, storage.ErrCacheHit) {
		return nil
	}

	if err != nil {
		return err
	}

	return compression.Decompress(&buf, vol.Mountpoint)
}

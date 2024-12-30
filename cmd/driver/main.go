package main

import (
	"os"

	"github.com/plexyhost/volume-driver/driver"
	"github.com/plexyhost/volume-driver/storage"

	"github.com/docker/go-plugins-helpers/volume"
	"github.com/sirupsen/logrus"
)

func main() {
	endpoint := "/live"
	if err := os.MkdirAll(endpoint, 0755); err != nil {
		logrus.Fatal(err)
	}

	store := storage.NewFSStorage("/storage")
	d := driver.NewNFSVolumeDriver(endpoint, store)
	h := volume.NewHandler(d)

	logrus.Info("Starting volume driver")
	if err := h.ServeUnix("plexvol", 0); err != nil {
		logrus.Fatal(err)
	}
}

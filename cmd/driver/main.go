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

	store, err := storage.NewHTTPStorage("http://192.168.0.170:30000/")
	if err != nil {
		panic(err)
	}
	d := driver.NewPlexVolumeDriver(endpoint, store)
	h := volume.NewHandler(d)

	logrus.Info("Starting volume driver")
	if err := h.ServeUnix("plexvol", 0); err != nil {
		logrus.Fatal(err)
	}
}

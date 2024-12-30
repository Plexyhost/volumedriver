package main

import (
	"os"

	"github.com/plexyhost/volume-driver/driver"

	"github.com/docker/go-plugins-helpers/volume"
	"github.com/sirupsen/logrus"
)

func main() {
	endpoint := "/mnt/serverdata"
	if err := os.MkdirAll(endpoint, 0755); err != nil {
		logrus.Fatal(err)
	}

	d := driver.NewNFSVolumeDriver(endpoint)
	h := volume.NewHandler(d)

	logrus.Info("Starting volume driver")
	if err := h.ServeUnix("plexvol", 0); err != nil {
		logrus.Fatal(err)
	}
}

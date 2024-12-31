package main

import (
	"flag"
	"os"

	"github.com/plexyhost/volume-driver/driver"
	"github.com/plexyhost/volume-driver/storage"

	"github.com/docker/go-plugins-helpers/volume"
	"github.com/sirupsen/logrus"
)

func main() {
	directory := flag.String("directory", "/live", "The folder where data from live servers are stored")
	endpoint := flag.String("endpoint", "", "The server which stores and retrieves server data")
	flag.Parse()

	if *endpoint == "" {
		panic("--endpoint must be set to a compatible server")
	}

	if err := os.MkdirAll(*directory, 0755); err != nil {
		logrus.Fatal(err)
	}

	store, err := storage.NewHTTPStorage(*endpoint)
	if err != nil {
		panic(err)
	}
	d := driver.NewPlexVolumeDriver(*directory, store)
	h := volume.NewHandler(d)

	logrus.Info("Starting volume driver")
	if err := h.ServeUnix("plexhost", 0); err != nil {
		logrus.Fatal(err)
	}
}

package main

import (
	"flag"
	"github.com/charmbracelet/log"
	"os"

	"github.com/plexyhost/volume-driver/driver"
	"github.com/plexyhost/volume-driver/storage"

	"github.com/docker/go-plugins-helpers/volume"
)

const (
	socketName = "plexhost"
)

func main() {
	directory := flag.String("directory", "/live", "The folder where data from live servers are stored")
	endpoint := flag.String("endpoint", "http://192.168.0.170:30000", "The server which stores and retrieves server data")
	flag.Parse()

	if *endpoint == "" {
		log.Fatal("--endpoint must be set to a compatible server")
	}

	if err := os.MkdirAll(*directory, 0755); err != nil {
		log.Fatal(err)
	}

	store, err := storage.NewHTTPStorage(*endpoint)
	if err != nil {
		log.Fatal(err)
	}

	d := driver.NewPlexVolumeDriver(*directory, store)
	h := volume.NewHandler(d)

	log.Info("Starting Plex volume driver...")

	if err := h.ServeUnix(socketName, 0); err != nil {
		log.Fatal("Failed to serve unix", "error", err)
	}
}

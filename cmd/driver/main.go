package main

import (
	"flag"
	"net/url"
	"os"

	"github.com/charmbracelet/log"

	"github.com/plexyhost/volume-driver/driver"
	"github.com/plexyhost/volume-driver/storage"

	"github.com/docker/go-plugins-helpers/volume"
)

const (
	socketName = "plexhost"
)

func main() {
	directory := flag.String("directory", "/live", "The folder where data from live servers are stored")
	flag.Parse()

	endpoint := os.Getenv("ENDPOINT")
	if endpoint == "" {
		log.Fatal("endpoint cannot be empty")
	}

	if err := os.MkdirAll(*directory, 0755); err != nil {
		log.Fatal(err)
	}

	if _, err := url.ParseRequestURI(endpoint); err != nil {
		log.Fatal(err)
	}

	store, err := storage.NewHTTPStorage(endpoint)
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

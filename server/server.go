package main

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"os"
)

func main() {
	m := http.NewServeMux()

	m.HandleFunc("GET /checksum/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		f, err := os.Open(id + ".plex")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer f.Close()

		hash := sha256.New()
		if _, err := f.WriteTo(hash); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "%x", hash.Sum(nil))
	})

	m.HandleFunc("GET /data/{id}", func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("getting...")
		id := r.PathValue("id")

		f, err := os.Open(id + ".plex")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer f.Close()

		w.Header().Add("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		f.WriteTo(w)
	})

	m.HandleFunc("PUT /data/{id}", func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("receiving...")
		id := r.PathValue("id")

		f, err := os.Open(id + ".plex")
		if errors.Is(err, os.ErrNotExist) {
			fmt.Printf("err1: %v\n", err)
			f, err = os.Create(id + ".plex")
		}

		if err != nil {
			fmt.Printf("err2: %v\n", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer f.Close()
		defer r.Body.Close()

		f.ReadFrom(r.Body)
		defer r.Body.Close()

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success!"))
	})

	err := http.ListenAndServe(":30000", m)
	if err != nil {
		panic(err)
	}
}

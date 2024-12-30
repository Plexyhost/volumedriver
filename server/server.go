package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
)

func main() {
	m := http.NewServeMux()

	m.HandleFunc("GET /{id}", func(w http.ResponseWriter, r *http.Request) {
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

	m.HandleFunc("PUT /{id}", func(w http.ResponseWriter, r *http.Request) {
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

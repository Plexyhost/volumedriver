package main

import (
	"net/http"
	"os"
)

func main() {
	m := http.NewServeMux()

	m.HandleFunc("GET /{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		f, err := os.Open(id + ".plex")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		f.ReadFrom(r.Body)
		defer r.Body.Close()

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success!"))
	})

	m.HandleFunc("PUT /{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		f, err := os.Open(id + ".plex")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer r.Body.Close()

		w.Header().Add("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		f.WriteTo(w)
	})

	err := http.ListenAndServe(":30000", m)
	if err != nil {
		panic(err)
	}
}

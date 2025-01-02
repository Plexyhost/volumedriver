package main

import (
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"math/rand/v2"
	"net/http"
	"os"
	"strconv"
)

// Function to handle file cleanup after the upload is complete
func finalizeFile(tempFilePath, finalFilePath string) error {
	// Rename the temporary file to the final file name
	err := os.Rename(tempFilePath, finalFilePath)
	if err != nil {
		return fmt.Errorf("failed to finalize file: %v", err)
	}
	fmt.Println("File finalized and saved successfully.")
	return nil
}

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
			log.Default().Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "%x", hash.Sum(nil))
	})

	m.HandleFunc("GET /data/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		log.Println("INIT STORAGE->DRIVER:" + id)

		f, err := os.Open(id + ".plex")
		if err != nil {
			log.Default().Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer f.Close()

		w.Header().Add("Content-Type", "binary/octet-stream")
		w.WriteHeader(http.StatusOK)
		n, err := f.WriteTo(w)
		if err != nil {
			log.Default().Println(err)
			return
		}

		log.Println("COMPLETED STORAGE->DRIVER:"+id, "\nWritten", n, "bytes.")
	})

	m.HandleFunc("PUT /data/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		log.Println("INIT DRIVER->STORAGE:" + id)

		fn := id + ".plex"
		tf := strconv.Itoa(rand.IntN(512)) + ".bin"

		outFile, err := os.OpenFile(tf, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			log.Default().Println(err)
			http.Error(w, "Could not create temporary file", http.StatusInternalServerError)
			return
		}
		defer outFile.Close()

		// Read the incoming file data from the request body and write it to the temporary file
		n, err := io.Copy(outFile, r.Body)
		if err != nil {
			log.Default().Println(err)
			http.Error(w, "Failed to save file chunk", http.StatusInternalServerError)
			return
		}

		// After the upload is complete, finalize the file
		// In a real use case, you would check if all chunks have been uploaded
		err = finalizeFile(tf, fn)
		if err != nil {
			log.Default().Println(err)
			http.Error(w, "Failed to finalize the file", http.StatusInternalServerError)
			return
		}

		// Respond with success
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("File uploaded and saved successfully"))
		log.Println("COMPLETED DRIVER->STORAGE:"+id, "\nReceived", n, "bytes.")
	})

	err := http.ListenAndServe(":30000", m)
	if err != nil {
		panic(err)
	}
}

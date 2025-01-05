package main

import (
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/charmbracelet/log"
)

// Function to handle file cleanup after the upload is complete
func finalizeFile(tempFilePath, finalFilePath string) error {
	// Rename the temporary file to the final file name
	err := os.Rename(tempFilePath, finalFilePath)
	if err != nil {
		return fmt.Errorf("failed to finalize file: %v", err)
	}
	return nil
}

func main() {
	m := http.NewServeMux()

	m.HandleFunc("GET /data/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		log.Info("INIT STORAGE->DRIVER", "id", id)
		start := time.Now()

		f, err := os.Open(id + ".plex")
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		defer f.Close()

		w.Header().Add("Content-Type", "binary/octet-stream")
		w.WriteHeader(http.StatusOK)
		n, err := f.WriteTo(w)
		if err != nil {
			log.Error("Occured an error under STORAGE->DRIVER", "id", id, "error", err)
			return
		}

		log.Info("COMPLETED STORAGE->DRIVER", "id", id, "bytes_written", byteCount(n), "took", time.Since(start))
	})

	m.HandleFunc("PUT /data/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		log.Info("INIT DRIVER->STORAGE", "id", id)
		start := time.Now()

		fn := id + ".plex"
		tf := strconv.Itoa(rand.IntN(512)) + ".bin"

		outFile, err := os.OpenFile(tf, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			log.Error("Occured an error under DRIVER->STORAGE", "id", id, "error", err)
			http.Error(w, "Could not create temporary file", http.StatusInternalServerError)
			return
		}
		defer outFile.Close()

		// Read the incoming file data from the request body and write it to the temporary file
		n, err := io.Copy(outFile, r.Body)
		if err != nil {
			log.Info("Failed to save chunk", "id", id, "error", err)
			http.Error(w, "Failed to save file chunk", http.StatusInternalServerError)
			return
		}

		// After the upload is complete, finalize the file
		// In a real use case, you would check if all chunks have been uploaded
		err = finalizeFile(tf, fn)
		if err != nil {
			log.Error("Failed to finalize file", "id", id, "error", err)
			http.Error(w, "Failed to finalize the file", http.StatusInternalServerError)
			return
		}

		// Respond with success
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("File uploaded and saved successfully"))
		log.Info("COMPLETED DRIVER->STORAGE", "id", id, "bytes_read", byteCount(n), "took", time.Since(start))
	})

	err := http.ListenAndServe(":3000", m)
	if err != nil {
		panic(err)
	}
}

func byteCount(b int64) string {
	const unit = 1000
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%cB",
		float64(b)/float64(div), "kMGTPE"[exp])
}

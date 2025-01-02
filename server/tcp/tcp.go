package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"math/rand/v2"
	"net"
	"os"
	"strconv"
	"strings"
)

// Function to handle file cleanup after the upload is complete
func finalizeFile(tempFilePath, finalFilePath string) error {
	if _, err := os.Stat(finalFilePath); err == nil {
		if removeErr := os.Remove(finalFilePath); removeErr != nil {
			return fmt.Errorf("failed to remove existing file: %v", removeErr)
		}
	}
	// Rename the temporary file to the final file name
	err := os.Rename(tempFilePath, finalFilePath)
	if err != nil {
		return fmt.Errorf("failed to finalize file: %v", err)
	}
	fmt.Println("File finalized and saved successfully.")
	return nil
}

func handleConn(conn net.Conn) {
	log.Println("New connection from", conn.RemoteAddr())
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.SetNoDelay(true)
		tcpConn.SetReadBuffer(65536)
		tcpConn.SetWriteBuffer(65536)
	}
	defer conn.Close()
	r := bufio.NewReader(conn)
	cmdLine, err := r.ReadString('\n')
	if err != nil {
		return
	}
	log.Println("Received command line:", cmdLine)
	parts := strings.Split(strings.TrimSpace(cmdLine), ":")
	fmt.Printf("len(parts): %v\n", len(parts))
	if len(parts) == 3 && parts[0] == "STORE" {
		id := parts[1]
		size, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil {
			log.Println("Invalid file size:", err)
			return
		}
		log.Println("Storing file with ID:", id, "Size:", size)
		tf := strconv.Itoa(rand.IntN(512)) + ".bin"
		outFile, err := os.OpenFile(tf, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			log.Println("Error:", err)
			return
		}
		defer outFile.Close()
		written, err := io.CopyN(outFile, r, size)
		if err != nil || written != size {
			log.Println("Error copying data:", err)
			return
		}
		if err := finalizeFile(tf, id+".plex"); err != nil {
			log.Println("Error finalizing file:", err)
			return
		}
		log.Println("File stored successfully for ID:", id)
		conn.Write([]byte("OK\n"))
	} else if len(parts) == 2 {
		switch parts[0] {
		case "RETRIEVE":
			id := parts[1]
			log.Println("Retrieving file with ID:", id)
			f, err := os.Open(id + ".plex")
			if err != nil {
				log.Println("Error:", err)
				return
			}
			defer f.Close()
			conn.Write([]byte("OK\n"))
			io.Copy(conn, f)
			log.Println("File retrieved successfully for ID:", id)
		}
	}
}

func main() {
	log.Println("Starting TCP server on :30000")
	ln, err := net.Listen("tcp", ":30000")
	if err != nil {
		panic(err)
	}
	defer ln.Close()
	println("tcp server listening on [::]:30000")
	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}
		go handleConn(conn)
	}
}

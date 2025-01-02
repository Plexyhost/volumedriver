package main

import (
	"bufio"
	"fmt"
	"io"
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
	parts := strings.Split(strings.TrimSpace(cmdLine), ":")
	if len(parts) == 2 {
		switch parts[0] {
		case "STORE":
			id := parts[1]
			tf := strconv.Itoa(rand.IntN(512)) + ".bin"
			outFile, _ := os.OpenFile(tf, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
			defer outFile.Close()
			io.Copy(outFile, r)
			finalizeFile(tf, id+".plex")
			conn.Write([]byte("OK\n"))
		case "RETRIEVE":
			id := parts[1]
			f, err := os.Open(id + ".plex")
			if err != nil {
				return
			}
			defer f.Close()
			conn.Write([]byte("OK\n"))
			io.Copy(conn, f)
		}
	}
}

func main() {
	ln, err := net.Listen("tcp", ":30000")
	if err != nil {
		panic(err)
	}
	defer ln.Close()

	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}
		go handleConn(conn)
	}
}

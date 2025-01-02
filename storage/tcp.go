package storage

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
)

type tcpStorage struct {
	endpoint *url.URL
}

func NewTCPStorage(endpoint string) (StorageProvider, error) {
	ep, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}

	return &tcpStorage{
		endpoint: ep,
	}, nil
}

func (ts *tcpStorage) Store(id string, src io.Reader) error {
	conn, err := net.Dial("tcp", ts.endpoint.Host)
	if err != nil {
		return err
	}
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.SetNoDelay(true)
		tcpConn.SetReadBuffer(65536)
		tcpConn.SetWriteBuffer(65536)
	}
	defer conn.Close()
	fmt.Fprintf(conn, "STORE:%s\n", id)
	_, err = io.Copy(conn, src)
	if err != nil {
		return err
	}
	res, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil || res != "OK\n" {
		return errors.New("failed to store over TCP")
	}
	return nil
}

func (ts *tcpStorage) Retrieve(id string, dst io.Writer) error {
	conn, err := net.Dial("tcp", ts.endpoint.Host)
	if err != nil {
		return err
	}
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.SetNoDelay(true)
		tcpConn.SetReadBuffer(65536)
		tcpConn.SetWriteBuffer(65536)
	}
	defer conn.Close()
	fmt.Fprintf(conn, "RETRIEVE:%s\n", id)
	res, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil || res != "OK\n" {
		return errors.New("failed to retrieve over TCP")
	}
	_, err = io.Copy(dst, conn)
	return err
}

package storage

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

type httpStorage struct {
	cl       *http.Client
	endpoint *url.URL
}

func NewHTTPStorage(endpoint string) (StorageProvider, error) {
	ep, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}

	return &httpStorage{
		cl:       &http.Client{},
		endpoint: ep,
	}, nil
}

// Must return
func (hs *httpStorage) Store(id string, src io.Reader) error {
	r, err := http.NewRequest("PUT", hs.endpoint.String(), src)
	if err != nil {
		return err
	}

	res, err := hs.cl.Do(r)
	if err != nil {
		return err
	}
	cnt, _ := io.ReadAll(res.Body)
	defer res.Body.Close()
	fmt.Printf("cnt: %v\n", string(cnt))

	if res.StatusCode == 200 {
		return nil
	}
	return errors.New("non-200 response from http storage provider")
}

func (hs *httpStorage) Retrieve(id string, dst io.Writer) error {
	ep := hs.endpoint.JoinPath(id).String()
	r, err := http.NewRequest("GET", ep, nil)
	if err != nil {
		return err
	}

	res, err := hs.cl.Do(r)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		return errors.New("non-200 response from http storage provider")
	}

	_, err = io.Copy(dst, res.Body)
	return err
}

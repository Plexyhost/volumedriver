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
	// checksums is a map that points any server id to the sum
}

func NewHTTPStorage(endpoint string) (Provider, error) {
	ep, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}

	return &httpStorage{
		cl:       &http.Client{},
		endpoint: ep,
	}, nil
}

func (hs *httpStorage) Store(id string, src io.Reader) error {
	ep := hs.endpoint.JoinPath("data", id)
	r, err := http.NewRequest("PUT", "", src)
	r.Header.Add("Content-Type", "binary/octet-stream")
	r.URL = ep
	if err != nil {
		return err
	}

	res, err := hs.cl.Do(r)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode == 200 {
		return nil
	}

	return errors.Join(ErrNon200, fmt.Errorf("code received: %d", res.StatusCode))
}

func (hs *httpStorage) Retrieve(id string, dst io.Writer) error {
	ep := hs.endpoint.JoinPath("data", id)
	r, err := http.NewRequest("GET", "", nil)
	r.URL = ep

	if err != nil {
		return err
	}

	res, err := hs.cl.Do(r)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		return errors.Join(ErrNon200, fmt.Errorf("code received: %d", res.StatusCode))
	}

	_, err = io.Copy(dst, res.Body)
	if err != nil {
		return err
	}

	return nil
}

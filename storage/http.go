package storage

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/charmbracelet/log"
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
	r, err := http.NewRequest("PUT", ep.String(), src)
	if err != nil {
		return err
	}
	r.Header.Add("Content-Type", "binary/octet-stream")

	res, err := hs.cl.Do(r)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode == 200 {
		return nil
	}

	d, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}

	return errors.Join(ErrNon200, fmt.Errorf("code received while storing: %d. Data: %s", res.StatusCode, string(d)))
}

func (hs *httpStorage) Retrieve(id string, dst io.Writer) error {
	ep := hs.endpoint.JoinPath("data", id)
	r, err := http.NewRequest("GET", ep.String(), nil)
	log.Info("GETTING", "ep", ep.String())

	if err != nil {
		return err
	}

	res, err := hs.cl.Do(r)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		if res.StatusCode == 404 {
			return nil
		}
		dat, err2 := io.ReadAll(res.Body)
		if err2 != nil {
			return err
		}
		return errors.Join(ErrNon200, fmt.Errorf("code received while retrieving: %d. Data: %s", res.StatusCode, string(dat)))
	}

	_, err = io.Copy(dst, res.Body)
	if err != nil {
		return err
	}

	return nil
}

package storage

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"sync"
)

type httpStorage struct {
	cl       *http.Client
	endpoint *url.URL
	// checksums is a map that points any server id to the sum
	checksums map[string]string
	mutex     *sync.Mutex
}

func NewHTTPStorage(endpoint string) (StorageProvider, error) {
	ep, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}

	return &httpStorage{
		cl:        &http.Client{},
		endpoint:  ep,
		checksums: make(map[string]string),
		mutex:     &sync.Mutex{},
	}, nil
}

// Must return
func (hs *httpStorage) Store(id string, src io.Reader) error {
	checksum, err := hs.getChecksum(id)
	if err != nil {
		return err
	}

	hs.mutex.Lock()
	existingChecksum, ok := hs.checksums[id]
	hs.checksums[id] = checksum
	hs.mutex.Unlock()
	if ok {
		if checksum == existingChecksum {
			return nil
		}
	}

	ep := hs.endpoint.JoinPath("data", id)
	r, err := http.NewRequest("PUT", "", src)
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

	return errors.New("non-200 response from http storage provider")
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
		return errors.New("non-200 response from http storage provider")
	}

	_, err = io.Copy(dst, res.Body)
	return err
}

func (hs *httpStorage) getChecksum(id string) (string, error) {
	ep := hs.endpoint.JoinPath("checksum", id)
	r, err := http.NewRequest("GET", "", nil)
	r.URL = ep

	if err != nil {
		return "", err
	}

	res, err := hs.cl.Do(r)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	checksum, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	return string(checksum), nil
}

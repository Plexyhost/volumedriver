package driver

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/docker/go-plugins-helpers/volume"
	"github.com/plexyhost/volume-driver/enc"
	"github.com/sirupsen/logrus"
)

type volumeInfo struct {
	ServerID string
	LastSync time.Time

	// Mountpoint is where the data will be saved locally
	mountpoint string

	ctx    context.Context
	cancel context.CancelFunc
}

type nfsVolumeDriver struct {
	volumes    map[string]*volumeInfo
	mutex      *sync.RWMutex
	endpoint   string
	syncPeriod time.Duration
}

func newNFSVolumeDriver(endpoint string) *nfsVolumeDriver {
	return &nfsVolumeDriver{
		volumes:    make(map[string]*volumeInfo),
		mutex:      &sync.RWMutex{},
		endpoint:   endpoint,
		syncPeriod: 5 * time.Minute,
	}
}

// req.Name __has__ to be the server's id.
func (d *nfsVolumeDriver) Create(req *volume.CreateRequest) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	logrus.WithField("name", req.Name).Info("Creating volume")

	mountpoint := filepath.Join(d.endpoint, req.Name)
	if err := os.MkdirAll(mountpoint, 0755); err != nil {
		return err
	}

	// TODO
	// Initial request of data
	// Unziping using gzip.Writer(req.Body)

	ctx, cancel := context.WithCancel(context.Background())

	d.volumes[req.Name] = &volumeInfo{
		ServerID: req.Name,
		LastSync: time.Now(),
		ctx:      ctx,
		cancel:   cancel,
	}

	// Start background sync for this volume
	go d.startPeriodicSave(ctx, req.Name)

	return nil
}

// TODO: Alt det her skal rykkes til unmount.
func (d *nfsVolumeDriver) Remove(req *volume.RemoveRequest) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	v, exists := d.volumes[req.Name]
	if !exists {
		return fmt.Errorf("volume %s not found", req.Name)
	}

	// TODO
	// Final sync to NFS before removal

	var buf bytes.Buffer

	enc.Compress(v.mountpoint, &buf)
	f, err := os.Open("/backups/" + v.ServerID + ".tar.gz")
	if err != nil {
		return err
	}
	n, err := buf.WriteTo(f)
	if err != nil {
		return err
	}
	fmt.Printf("bytes written: %v\n", n)

	if err := os.RemoveAll(v.mountpoint); err != nil {
		return err
	}

	delete(d.volumes, req.Name)
	return nil
}

func (d *nfsVolumeDriver) Path(req *volume.PathRequest) (*volume.PathResponse, error) {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	v, exists := d.volumes[req.Name]
	if !exists {
		return nil, fmt.Errorf("volume %s not found", req.Name)
	}

	return &volume.PathResponse{Mountpoint: v.mountpoint}, nil
}

func (d *nfsVolumeDriver) Mount(req *volume.MountRequest) (*volume.MountResponse, error) {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	v, exists := d.volumes[req.Name]
	if !exists {
		return nil, fmt.Errorf("volume %s not found", req.Name)
	}

	return &volume.MountResponse{Mountpoint: v.mountpoint}, nil
}

func (d *nfsVolumeDriver) Unmount(req *volume.UnmountRequest) error {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	_, exists := d.volumes[req.Name]
	if !exists {
		return fmt.Errorf("volume %s not found", req.Name)
	}

	return nil
}

func (d *nfsVolumeDriver) Get(req *volume.GetRequest) (*volume.GetResponse, error) {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	v, exists := d.volumes[req.Name]
	if !exists {
		return nil, fmt.Errorf("volume %s not found", req.Name)
	}

	return &volume.GetResponse{
		Volume: &volume.Volume{
			Name:       v.ServerID,
			Mountpoint: v.mountpoint,
		},
	}, nil
}

func (d *nfsVolumeDriver) List() (*volume.ListResponse, error) {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	var vols []*volume.Volume
	for _, v := range d.volumes {
		vols = append(vols, &volume.Volume{
			Name:       v.ServerID,
			Mountpoint: v.mountpoint,
		})
	}
	return &volume.ListResponse{Volumes: vols}, nil
}

func (d *nfsVolumeDriver) Capabilities() *volume.CapabilitiesResponse {
	return &volume.CapabilitiesResponse{
		Capabilities: volume.Capability{Scope: "local"},
	}
}

func (d *nfsVolumeDriver) startPeriodicSave(ctx context.Context, volumeName string) {
	ticker := time.NewTicker(d.syncPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			d.mutex.RLock()
			vol, exists := d.volumes[volumeName]
			d.mutex.RUnlock()

			if !exists {
				return
			}

			// TODO
			// Gzip
			// Send to http data server

		case <-ctx.Done():
			return
		}
	}
}

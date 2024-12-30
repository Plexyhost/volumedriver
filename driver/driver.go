package driver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/plexyhost/volume-driver/cmp"
	"github.com/plexyhost/volume-driver/storage"

	"github.com/docker/go-plugins-helpers/volume"
	"github.com/sirupsen/logrus"
)

// mangler i memory ved genstart af driver
type volumeInfo struct {
	ServerID string

	// Mountpoint is where the data will be saved locally
	Mountpoint string

	lastSync time.Time
	ctx      context.Context
	cancel   context.CancelFunc
}

type nfsVolumeDriver struct {
	Volumes    map[string]*volumeInfo
	mutex      *sync.RWMutex
	endpoint   string
	syncPeriod time.Duration
	store      storage.StorageProvider
}

func (d *nfsVolumeDriver) saveVolumes() error {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	file, err := os.Create(filepath.Join(d.endpoint, "volumes.json"))
	if err != nil {
		return err
	}
	defer file.Close()

	return json.NewEncoder(file).Encode(d.Volumes)
}

func (d *nfsVolumeDriver) loadVolumes() error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	file, err := os.Open(filepath.Join(d.endpoint, "volumes.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()

	return json.NewDecoder(file).Decode(&d.Volumes)
}

func NewNFSVolumeDriver(endpoint string, store storage.StorageProvider) *nfsVolumeDriver {

	driver := &nfsVolumeDriver{
		Volumes:    make(map[string]*volumeInfo),
		mutex:      &sync.RWMutex{},
		endpoint:   endpoint,
		syncPeriod: 2 * time.Minute,
		store:      store,
	}
	if err := driver.loadVolumes(); err != nil {
		logrus.WithError(err).Error("failed to load volumes")
	}
	return driver
}

// req.Name __has__ to be the server's id.
func (d *nfsVolumeDriver) Create(req *volume.CreateRequest) error {

	logrus.WithField("name", req.Name).Info("Creating volume")

	mountpoint := filepath.Join(d.endpoint, req.Name)
	fmt.Printf("mountpoint: %v\n", mountpoint)
	if err := os.MkdirAll(mountpoint, 0755); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())

	d.mutex.Lock()
	volInfo := &volumeInfo{
		ServerID:   req.Name,
		lastSync:   time.Now(),
		Mountpoint: mountpoint,
		ctx:        ctx,
		cancel:     cancel,
	}
	d.Volumes[req.Name] = volInfo
	d.mutex.Unlock()

	// Load store
	d.loadFromStore(volInfo)

	// Start background sync for this volume
	go d.startPeriodicSave(ctx, req.Name)

	if err := d.saveVolumes(); err != nil {
		logrus.WithError(err).Error("failed to save volumes")
	}

	return nil
}

// TODO: Alt det her skal rykkes til unmount.
func (d *nfsVolumeDriver) Remove(req *volume.RemoveRequest) error {

	v, exists := d.Volumes[req.Name]
	if !exists {
		return fmt.Errorf("volume %s not found", req.Name)
	}

	// Write last time to

	err := d.saveToStore(v)
	if err != nil {
		return err
	}

	v.cancel()

	if err := os.RemoveAll(v.Mountpoint); err != nil {
		return err
	}

	d.mutex.Lock()
	delete(d.Volumes, req.Name)
	d.mutex.Unlock()

	if err := d.saveVolumes(); err != nil {
		logrus.WithError(err).Error("failed to save volumes")
	}

	return nil
}

func (d *nfsVolumeDriver) Path(req *volume.PathRequest) (*volume.PathResponse, error) {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	v, exists := d.Volumes[req.Name]
	if !exists {
		return nil, fmt.Errorf("volume %s not found", req.Name)
	}

	return &volume.PathResponse{Mountpoint: v.Mountpoint}, nil
}

func (d *nfsVolumeDriver) Mount(req *volume.MountRequest) (*volume.MountResponse, error) {
	logrus.WithField("name", req.Name).Info("mounting driver")
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	v, exists := d.Volumes[req.Name]
	if !exists {
		logrus.Error("volume not found?")
		return nil, fmt.Errorf("volume %s not found", req.Name)
	}

	fmt.Printf("v.mountpoint: %v\n", v.Mountpoint)

	return &volume.MountResponse{Mountpoint: v.Mountpoint}, nil
}

func (d *nfsVolumeDriver) Unmount(req *volume.UnmountRequest) error {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	_, exists := d.Volumes[req.Name]
	if !exists {
		return fmt.Errorf("volume %s not found", req.Name)
	}

	fmt.Printf("unmount::req.Name: %v\n", req.Name)

	return nil
}

func (d *nfsVolumeDriver) Get(req *volume.GetRequest) (*volume.GetResponse, error) {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	v, exists := d.Volumes[req.Name]
	if !exists {
		return nil, fmt.Errorf("volume %s not found", req.Name)
	}

	return &volume.GetResponse{
		Volume: &volume.Volume{
			Name:       v.ServerID,
			Mountpoint: v.Mountpoint,
		},
	}, nil
}

func (d *nfsVolumeDriver) List() (*volume.ListResponse, error) {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	var vols []*volume.Volume
	for _, v := range d.Volumes {
		vols = append(vols, &volume.Volume{
			Name:       v.ServerID,
			Mountpoint: v.Mountpoint,
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
			v, exists := d.Volumes[volumeName]
			d.mutex.RUnlock()

			if !exists {
				return
			}

			logrus.WithField("id", v.ServerID).Info("syncing...")

			d.saveToStore(v)

		case <-ctx.Done():
			logrus.WithField("name", volumeName).Info("volume context ended, ending periodic save")
			return
		}
	}
}

func (d *nfsVolumeDriver) saveToStore(vol *volumeInfo) error {
	var buf bytes.Buffer

	err := cmp.Compress(vol.Mountpoint, &buf)
	if err != nil {
		return err
	}

	fmt.Println("compressed all")

	return d.store.Store(vol.ServerID, &buf)
}

func (d *nfsVolumeDriver) loadFromStore(vol *volumeInfo) error {
	var buf bytes.Buffer

	err := d.store.Retrieve(vol.ServerID, &buf)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}

	if err != nil {
		return err
	}

	return cmp.Decompress(&buf, vol.Mountpoint)
}

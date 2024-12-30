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

type plexVolumeDriver struct {
	Volumes        map[string]*volumeInfo
	mutex          *sync.RWMutex
	endpoint       string
	syncPeriod     time.Duration
	store          storage.StorageProvider
	volumeInfoPath string
}

func (d *plexVolumeDriver) saveVolumes() error {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	file, err := os.Create(filepath.Join(d.endpoint, d.volumeInfoPath))
	if err != nil {
		return err
	}
	defer file.Close()

	return json.NewEncoder(file).Encode(d.Volumes)
}

func (d *plexVolumeDriver) loadVolumes() error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	file, err := os.Open(filepath.Join(d.endpoint, d.volumeInfoPath))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()

	err = json.NewDecoder(file).Decode(&d.Volumes)
	if err != nil {
		return err
	}

	for _, v := range d.Volumes {
		v.ctx, v.cancel = context.WithCancel(context.Background())
		v.lastSync = time.Now()

		go d.startPeriodicSave(v.ctx, v.ServerID)
	}

	return nil
}

func NewPlexVolumeDriver(endpoint string, store storage.StorageProvider) *plexVolumeDriver {

	driver := &plexVolumeDriver{
		Volumes:        make(map[string]*volumeInfo),
		mutex:          &sync.RWMutex{},
		endpoint:       endpoint,
		syncPeriod:     2 * time.Minute,
		store:          store,
		volumeInfoPath: "volumes.json",
	}
	if err := driver.loadVolumes(); err != nil {
		logrus.WithError(err).Error("failed to load volumes")
	}
	return driver
}

// req.Name __has__ to be the server's id.
func (d *plexVolumeDriver) Create(req *volume.CreateRequest) error {

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
func (d *plexVolumeDriver) Remove(req *volume.RemoveRequest) error {

	// Get volume
	v, exists := d.Volumes[req.Name]
	if !exists {
		return fmt.Errorf("volume %s not found", req.Name)
	}

	// Remove data from the disk
	if err := os.RemoveAll(v.Mountpoint); err != nil {
		return err
	}

	// Remove the volume info from d.Volumes
	d.mutex.Lock()
	delete(d.Volumes, req.Name)
	d.mutex.Unlock()

	if err := d.saveVolumes(); err != nil {
		logrus.WithError(err).Error("failed to save volumes")
	}

	return nil
}

func (d *plexVolumeDriver) Path(req *volume.PathRequest) (*volume.PathResponse, error) {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	v, exists := d.Volumes[req.Name]
	if !exists {
		return nil, fmt.Errorf("volume %s not found", req.Name)
	}

	return &volume.PathResponse{Mountpoint: v.Mountpoint}, nil
}

func (d *plexVolumeDriver) Mount(req *volume.MountRequest) (*volume.MountResponse, error) {
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

func (d *plexVolumeDriver) Unmount(req *volume.UnmountRequest) error {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	v, exists := d.Volumes[req.Name]
	if !exists {
		return fmt.Errorf("volume %s not found", req.Name)
	}

	//

	err := d.saveToStore(v)
	if err != nil {
		return err
	}

	v.cancel()

	return nil
}

func (d *plexVolumeDriver) Get(req *volume.GetRequest) (*volume.GetResponse, error) {
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

func (d *plexVolumeDriver) List() (*volume.ListResponse, error) {
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

func (d *plexVolumeDriver) Capabilities() *volume.CapabilitiesResponse {
	return &volume.CapabilitiesResponse{
		Capabilities: volume.Capability{Scope: "local"},
	}
}

func (d *plexVolumeDriver) startPeriodicSave(ctx context.Context, volumeName string) {
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

func (d *plexVolumeDriver) saveToStore(vol *volumeInfo) error {
	var buf bytes.Buffer

	err := cmp.Compress(vol.Mountpoint, &buf)
	if err != nil {
		return err
	}

	fmt.Println("compressed all")

	return d.store.Store(vol.ServerID, &buf)
}

func (d *plexVolumeDriver) loadFromStore(vol *volumeInfo) error {
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

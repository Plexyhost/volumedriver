package driver

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/plexyhost/volume-driver/storage"

	"github.com/docker/go-plugins-helpers/volume"
	"github.com/sirupsen/logrus"
)

// mangler i memory ved genstart af driver
type volumeInfo struct {
	ServerID string
	Mounted  bool

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
		if v.Mounted {
			go d.startPeriodicSave(v.ctx, v.ServerID)
		}
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

	d.mutex.Lock()
	volInfo := &volumeInfo{
		ServerID:   req.Name,
		Mountpoint: mountpoint,
		Mounted:    false,
		lastSync:   time.Now(),
		ctx:        nil,
		cancel:     nil,
	}
	d.Volumes[req.Name] = volInfo
	d.mutex.Unlock()

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

	// Find volume
	d.mutex.RLock()
	v, exists := d.Volumes[req.Name]
	if !exists {
		logrus.Error("volume not found?")
		return nil, fmt.Errorf("volume %s not found", req.Name)
	}
	d.mutex.RUnlock()

	// Load store
	d.loadFromStore(v)

	// Set mounted and context stuff
	d.mutex.Lock()
	v.Mounted = true
	v.ctx, v.cancel = context.WithCancel(context.Background())
	d.mutex.Unlock()

	// Start background sync for this volume
	go d.startPeriodicSave(v.ctx, v.ServerID)

	// Save volumes to disk for persisency
	if err := d.saveVolumes(); err != nil {
		logrus.WithError(err).Error("failed to save volumes")
	}

	return &volume.MountResponse{Mountpoint: v.Mountpoint}, nil
}

func (d *plexVolumeDriver) Unmount(req *volume.UnmountRequest) error {
	logrus.WithField("name", req.Name).Info("unmounting driver")

	d.mutex.RLock()
	v, exists := d.Volumes[req.Name]
	if !exists {
		return fmt.Errorf("volume %s not found", req.Name)
	}
	d.mutex.RUnlock()

	fmt.Println("unmount triggered save to store")
	err := d.saveToStore(v)
	if err != nil {
		return err
	}

	// Cancel context and set mounted to false
	d.mutex.Lock()
	v.cancel()
	v.Mounted = false
	d.mutex.Unlock()

	// Save volumes to disk for persisency
	if err := d.saveVolumes(); err != nil {
		logrus.WithError(err).Error("failed to save volumes")
	}

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

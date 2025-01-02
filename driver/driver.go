package driver

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/docker/go-plugins-helpers/volume"
	"github.com/plexyhost/volume-driver/storage"
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

type PlexVolumeDriver struct {
	Volumes        map[string]*volumeInfo
	mutex          *sync.RWMutex
	endpoint       string
	syncPeriod     time.Duration
	store          storage.Provider
	volumeInfoPath string
}

func (d *PlexVolumeDriver) saveVolumes() error {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	file, err := os.Create(filepath.Join(d.endpoint, d.volumeInfoPath))
	if err != nil {
		return err
	}
	defer file.Close()

	return json.NewEncoder(file).Encode(d.Volumes)
}

func (d *PlexVolumeDriver) loadVolumes() error {
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

func NewPlexVolumeDriver(endpoint string, store storage.Provider) *PlexVolumeDriver {

	driver := &PlexVolumeDriver{
		Volumes:        make(map[string]*volumeInfo),
		mutex:          &sync.RWMutex{},
		endpoint:       endpoint,
		syncPeriod:     4 * time.Minute,
		store:          store,
		volumeInfoPath: "volumes.json",
	}
	if err := driver.loadVolumes(); err != nil {
		log.Info("Failed to save volumes", "error", err)
	}
	return driver
}

// req.Name __has__ to be the server's id.
func (d *PlexVolumeDriver) Create(req *volume.CreateRequest) error {

	log.Info("Creating volume", "name", req.Name)

	mountpoint := filepath.Join(d.endpoint, req.Name)
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
		log.Info("Failed to save volumes", "error", err)
		return err
	}

	return nil
}

func (d *PlexVolumeDriver) Remove(req *volume.RemoveRequest) error {

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
		log.Info("Failed to save volumes", "error", err)
		return err
	}

	return nil
}

func (d *PlexVolumeDriver) Path(req *volume.PathRequest) (*volume.PathResponse, error) {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	v, exists := d.Volumes[req.Name]
	if !exists {
		return nil, fmt.Errorf("volume %s not found", req.Name)
	}

	return &volume.PathResponse{Mountpoint: v.Mountpoint}, nil
}

func (d *PlexVolumeDriver) Mount(req *volume.MountRequest) (*volume.MountResponse, error) {
	log.Info("Mounting volume", "name", req.Name)

	// Find volume
	d.mutex.RLock()
	v, exists := d.Volumes[req.Name]
	if !exists {
		log.Warn("Volume not found??")
		return nil, fmt.Errorf("volume %s not found", req.Name)
	}
	d.mutex.RUnlock()

	// Load store
	err := d.loadFromStore(v)
	if err != nil {
		return nil, err
	}

	// Set mounted and context stuff
	d.mutex.Lock()
	v.Mounted = true
	v.ctx, v.cancel = context.WithCancel(context.Background())
	d.mutex.Unlock()

	// Start background sync for this volume
	go d.startPeriodicSave(v.ctx, v.ServerID)

	// Save volumes to disk for persisency
	if err := d.saveVolumes(); err != nil {
		log.Error("Failed to save volumes", "error", err)
		return nil, err
	}

	return &volume.MountResponse{Mountpoint: v.Mountpoint}, nil
}

func (d *PlexVolumeDriver) Unmount(req *volume.UnmountRequest) error {
	log.Info("Unmounting driver...", "name", req.Name)

	d.mutex.RLock()
	v, exists := d.Volumes[req.Name]
	if !exists {
		return fmt.Errorf("volume %s not found", req.Name)
	}
	v.cancel()
	d.mutex.RUnlock()

	log.Info("Saving volume to store", req.Name)
	err := d.saveToStore(v)
	if err != nil {
		return err
	}

	// Cancel context and set mounted to false
	d.mutex.Lock()
	v.Mounted = false
	d.mutex.Unlock()

	// Save volumes to disk for persistency
	if err := d.saveVolumes(); err != nil {
		log.Error("Failed to save volumes")
	}

	return nil
}

func (d *PlexVolumeDriver) Get(req *volume.GetRequest) (*volume.GetResponse, error) {
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

func (d *PlexVolumeDriver) List() (*volume.ListResponse, error) {
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

func (d *PlexVolumeDriver) Capabilities() *volume.CapabilitiesResponse {
	return &volume.CapabilitiesResponse{
		Capabilities: volume.Capability{Scope: "local"},
	}
}

func (d *PlexVolumeDriver) startPeriodicSave(ctx context.Context, volumeName string) {
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
			log.Debug("Syncing volume", "id", v.ServerID)

			err := d.saveToStore(v)
			if err != nil {
				log.Error("Failed to sync volume periodically", "error", err)
			}

		case <-ctx.Done():
			log.Info("Volume context exceeded, stopping periodic save.")
			return
		}
	}
}

package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"termbridge-go/internal/identity"
)

const DeviceFileName = "device.json"

type Device struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type DeviceOptions struct {
	StateDir   string
	DeviceName string
	IDs        identity.Generator
	Now        func() time.Time
	Hostname   func() (string, error)
}

func LoadOrCreateDevice(options DeviceOptions) (Device, error) {
	if strings.TrimSpace(options.StateDir) == "" {
		return Device{}, fmt.Errorf("state dir is required")
	}
	if options.IDs == nil {
		options.IDs = identity.NewULIDGenerator()
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	if options.Hostname == nil {
		options.Hostname = os.Hostname
	}
	path := filepath.Join(options.StateDir, DeviceFileName)
	device, err := readDevice(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return Device{}, err
		}
		device, err = newDevice(options)
		if err != nil {
			return Device{}, err
		}
	}
	if name := strings.TrimSpace(options.DeviceName); name != "" {
		device.Name = name
	}
	if err := writeDevice(path, device); err != nil {
		return Device{}, err
	}
	return device, nil
}

func readDevice(path string) (Device, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Device{}, err
	}
	var device Device
	if err := json.Unmarshal(data, &device); err != nil {
		return Device{}, fmt.Errorf("read device identity: %w", err)
	}
	if strings.TrimSpace(device.ID) == "" {
		return Device{}, fmt.Errorf("read device identity: missing id")
	}
	if strings.TrimSpace(device.Name) == "" {
		return Device{}, fmt.Errorf("read device identity: missing name")
	}
	if device.CreatedAt.IsZero() {
		return Device{}, fmt.Errorf("read device identity: missing created_at")
	}
	return device, nil
}

func newDevice(options DeviceOptions) (Device, error) {
	id, err := options.IDs.NewID()
	if err != nil {
		return Device{}, fmt.Errorf("generate device id: %w", err)
	}
	name := strings.TrimSpace(options.DeviceName)
	if name == "" {
		name, err = options.Hostname()
		if err != nil || strings.TrimSpace(name) == "" {
			name = "termbridge-device"
		}
	}
	return Device{ID: id, Name: name, CreatedAt: options.Now().UTC()}, nil
}

func writeDevice(path string, device Device) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(device, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

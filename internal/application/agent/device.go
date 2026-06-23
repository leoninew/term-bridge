package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const DeviceFileName = "device.json"

type Device struct {
	Id        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type DeviceOptions struct {
	StateDir   string
	DeviceId   string
	DeviceName string
	Now        func() time.Time
}

func LoadOrCreateDevice(options DeviceOptions) (Device, error) {
	if strings.TrimSpace(options.StateDir) == "" {
		return Device{}, fmt.Errorf("state dir is required")
	}
	deviceId := strings.TrimSpace(options.DeviceId)
	if deviceId == "" {
		return Device{}, fmt.Errorf("device id is required")
	}
	deviceName := strings.TrimSpace(options.DeviceName)
	if deviceName == "" {
		return Device{}, fmt.Errorf("device name is required")
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	now := options.Now().UTC()
	path := filepath.Join(options.StateDir, "devices", safeDeviceSegment(deviceId), DeviceFileName)
	device, err := readDevice(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return Device{}, err
		}
		device = Device{Id: deviceId, Name: deviceName, CreatedAt: now}
	}
	device.Id = deviceId
	device.Name = deviceName
	if device.CreatedAt.IsZero() {
		device.CreatedAt = now
	}
	device.UpdatedAt = now
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
	if strings.TrimSpace(device.Id) == "" {
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

func safeDeviceSegment(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "." || value == ".." {
		return "unknown"
	}
	value = strings.ReplaceAll(value, string(os.PathSeparator), "_")
	value = strings.ReplaceAll(value, "/", "_")
	return value
}

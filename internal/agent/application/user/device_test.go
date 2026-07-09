package application

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDeviceIdentityDoesNotPersistCloudBinding(t *testing.T) {
	stateDir := t.TempDir()
	createdAt := time.Date(2026, 7, 5, 10, 0, 0, 0, time.UTC)
	device, err := LoadOrCreateDevice(DeviceOptions{StateDir: stateDir, Now: func() time.Time { return createdAt }})
	if err != nil {
		t.Fatalf("LoadOrCreateDevice() error = %v", err)
	}

	loaded, err := LoadOrCreateDevice(DeviceOptions{StateDir: stateDir, Now: func() time.Time { return createdAt.Add(time.Hour) }})
	if err != nil {
		t.Fatalf("LoadOrCreateDevice(reload) error = %v", err)
	}
	if loaded.Id != device.Id || loaded.Name != device.Name || loaded.PublicKey != device.PublicKey {
		t.Fatalf("device identity changed after reload: before=%#v after=%#v", device, loaded)
	}

	data, err := os.ReadFile(filepath.Join(stateDir, DeviceIdentityFileName))
	if err != nil {
		t.Fatalf("read device identity file error = %v", err)
	}
	if strings.Contains(string(data), "cloud_binding") {
		t.Fatalf("device identity persisted cloud binding: %s", string(data))
	}
}

func TestLegacyCloudBindingIsIgnoredWhenLoadingDeviceIdentity(t *testing.T) {
	stateDir := t.TempDir()
	createdAt := time.Date(2026, 7, 5, 10, 0, 0, 0, time.UTC)
	device, err := LoadOrCreateDevice(DeviceOptions{StateDir: stateDir, Now: func() time.Time { return createdAt }})
	if err != nil {
		t.Fatalf("LoadOrCreateDevice() error = %v", err)
	}
	path := filepath.Join(stateDir, DeviceIdentityFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read device identity file error = %v", err)
	}
	legacy := strings.TrimRight(string(data), "\n}") + `,
  "cloud_binding": {
    "public_url": "https://cloud.example.test",
    "device_id": "dev-1",
    "device_name": "legacy-device",
    "connected_at": "2026-07-05T11:00:00Z"
  }
}
`
	if err := os.WriteFile(path, []byte(legacy), 0o600); err != nil {
		t.Fatalf("write legacy device identity error = %v", err)
	}

	loaded, err := LoadOrCreateDevice(DeviceOptions{StateDir: stateDir, Now: func() time.Time { return createdAt.Add(time.Hour) }})
	if err != nil {
		t.Fatalf("LoadOrCreateDevice(legacy) error = %v", err)
	}
	if loaded.Id != device.Id || loaded.Name != device.Name || loaded.PublicKey != device.PublicKey {
		t.Fatalf("legacy cloud binding changed device identity: before=%#v after=%#v", device, loaded)
	}
}

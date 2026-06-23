package agent

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadOrCreateDeviceCreatesConfigIdentity(t *testing.T) {
	stateDir := t.TempDir()
	now := time.Date(2026, 6, 20, 1, 2, 3, 0, time.UTC)
	device, err := LoadOrCreateDevice(DeviceOptions{StateDir: stateDir, DeviceId: "dev-1", DeviceName: "host-a", Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("LoadOrCreateDevice() error = %v", err)
	}
	if device.Id != "dev-1" || device.Name != "host-a" || !device.CreatedAt.Equal(now) || !device.UpdatedAt.Equal(now) {
		t.Fatalf("device = %#v", device)
	}
	if _, err := os.Stat(filepath.Join(stateDir, "devices", "dev-1", DeviceFileName)); err != nil {
		t.Fatalf("device file missing: %v", err)
	}
}

func TestLoadOrCreateDeviceUpdatesConfiguredName(t *testing.T) {
	stateDir := t.TempDir()
	createdAt := time.Date(2026, 6, 20, 1, 2, 3, 0, time.UTC)
	updatedAt := createdAt.Add(time.Hour)
	first, err := LoadOrCreateDevice(DeviceOptions{StateDir: stateDir, DeviceId: "dev-1", DeviceName: "host-a", Now: func() time.Time { return createdAt }})
	if err != nil {
		t.Fatalf("first LoadOrCreateDevice() error = %v", err)
	}
	second, err := LoadOrCreateDevice(DeviceOptions{StateDir: stateDir, DeviceId: "dev-1", DeviceName: "custom", Now: func() time.Time { return updatedAt }})
	if err != nil {
		t.Fatalf("second LoadOrCreateDevice() error = %v", err)
	}
	if second.Id != first.Id || second.Name != "custom" || !second.CreatedAt.Equal(first.CreatedAt) || !second.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("second = %#v, first = %#v", second, first)
	}
}

func TestLoadOrCreateDeviceRejectsMissingConfig(t *testing.T) {
	_, err := LoadOrCreateDevice(DeviceOptions{StateDir: t.TempDir(), DeviceName: "local"})
	if err == nil {
		t.Fatal("LoadOrCreateDevice() error = nil, want error")
	}
}

func TestLoadOrCreateDeviceRejectsBadFile(t *testing.T) {
	stateDir := t.TempDir()
	path := filepath.Join(stateDir, "devices", "dev-1", DeviceFileName)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte("not json"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	_, err := LoadOrCreateDevice(DeviceOptions{StateDir: stateDir, DeviceId: "dev-1", DeviceName: "local"})
	if err == nil {
		t.Fatal("LoadOrCreateDevice() error = nil, want error")
	}
}

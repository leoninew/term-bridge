package agent

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

type fixedIDs struct {
	id string
}

func (f fixedIDs) NewID() (string, error) {
	return f.id, nil
}

func TestLoadOrCreateDeviceCreatesNewIdentity(t *testing.T) {
	stateDir := t.TempDir()
	now := time.Date(2026, 6, 20, 1, 2, 3, 0, time.UTC)
	device, err := LoadOrCreateDevice(DeviceOptions{StateDir: stateDir, IDs: fixedIDs{id: "dev-1"}, Now: func() time.Time { return now }, Hostname: func() (string, error) { return "host-a", nil }})
	if err != nil {
		t.Fatalf("LoadOrCreateDevice() error = %v", err)
	}
	if device.ID != "dev-1" || device.Name != "host-a" || !device.CreatedAt.Equal(now) {
		t.Fatalf("device = %#v", device)
	}
	if _, err := os.Stat(filepath.Join(stateDir, DeviceFileName)); err != nil {
		t.Fatalf("device file missing: %v", err)
	}
}

func TestLoadOrCreateDeviceReadsStableIdentity(t *testing.T) {
	stateDir := t.TempDir()
	now := time.Date(2026, 6, 20, 1, 2, 3, 0, time.UTC)
	first, err := LoadOrCreateDevice(DeviceOptions{StateDir: stateDir, IDs: fixedIDs{id: "dev-1"}, Now: func() time.Time { return now }, Hostname: func() (string, error) { return "host-a", nil }})
	if err != nil {
		t.Fatalf("first LoadOrCreateDevice() error = %v", err)
	}
	second, err := LoadOrCreateDevice(DeviceOptions{StateDir: stateDir, IDs: fixedIDs{id: "dev-2"}, Now: func() time.Time { return now.Add(time.Hour) }, Hostname: func() (string, error) { return "host-b", nil }})
	if err != nil {
		t.Fatalf("second LoadOrCreateDevice() error = %v", err)
	}
	if second.ID != first.ID || second.Name != first.Name || !second.CreatedAt.Equal(first.CreatedAt) {
		t.Fatalf("second = %#v, first = %#v", second, first)
	}
}

func TestLoadOrCreateDeviceOverridesName(t *testing.T) {
	stateDir := t.TempDir()
	now := time.Date(2026, 6, 20, 1, 2, 3, 0, time.UTC)
	first, err := LoadOrCreateDevice(DeviceOptions{StateDir: stateDir, IDs: fixedIDs{id: "dev-1"}, Now: func() time.Time { return now }, Hostname: func() (string, error) { return "host-a", nil }})
	if err != nil {
		t.Fatalf("first LoadOrCreateDevice() error = %v", err)
	}
	second, err := LoadOrCreateDevice(DeviceOptions{StateDir: stateDir, DeviceName: "custom", IDs: fixedIDs{id: "dev-2"}, Now: func() time.Time { return now.Add(time.Hour) }, Hostname: func() (string, error) { return "host-b", nil }})
	if err != nil {
		t.Fatalf("second LoadOrCreateDevice() error = %v", err)
	}
	if second.ID != first.ID || second.Name != "custom" || !second.CreatedAt.Equal(first.CreatedAt) {
		t.Fatalf("second = %#v", second)
	}
}

func TestLoadOrCreateDeviceRejectsBadFile(t *testing.T) {
	stateDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(stateDir, DeviceFileName), []byte("not json"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	_, err := LoadOrCreateDevice(DeviceOptions{StateDir: stateDir, IDs: fixedIDs{id: "dev-1"}})
	if err == nil {
		t.Fatal("LoadOrCreateDevice() error = nil, want error")
	}
}

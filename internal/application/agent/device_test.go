package agent

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadOrCreateDeviceCreatesStateIdentity(t *testing.T) {
	stateDir := t.TempDir()
	now := time.Date(2026, 6, 20, 1, 2, 3, 0, time.UTC)
	device, err := LoadOrCreateDevice(DeviceOptions{StateDir: stateDir, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("LoadOrCreateDevice() error = %v", err)
	}
	if device.Id == "" || device.Name != defaultDeviceName() || !device.CreatedAt.Equal(now) || !device.UpdatedAt.Equal(now) {
		t.Fatalf("device = %#v", device)
	}
	if _, err := os.Stat(filepath.Join(stateDir, DeviceIdentityFileName)); err != nil {
		t.Fatalf("identity file missing: %v", err)
	}
	deviceDir := filepath.Join(stateDir, "devices", device.Id)
	if _, err := os.Stat(filepath.Join(deviceDir, PrivateKeyFileName)); err != nil {
		t.Fatalf("private key file missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(deviceDir, PublicKeyFileName)); err != nil {
		t.Fatalf("public key file missing: %v", err)
	}
	if device.SchemaVersion != DeviceSchemaVersion || device.PublicKey == "" {
		t.Fatalf("device key metadata missing: %#v", device)
	}
}

func TestLoadOrCreateDeviceLoadsStateIdentityAndKeepsKey(t *testing.T) {
	stateDir := t.TempDir()
	createdAt := time.Date(2026, 6, 20, 1, 2, 3, 0, time.UTC)
	updatedAt := createdAt.Add(time.Hour)
	first, err := LoadOrCreateDevice(DeviceOptions{StateDir: stateDir, Now: func() time.Time { return createdAt }})
	if err != nil {
		t.Fatalf("first LoadOrCreateDevice() error = %v", err)
	}
	second, err := LoadOrCreateDevice(DeviceOptions{StateDir: stateDir, Now: func() time.Time { return updatedAt }})
	if err != nil {
		t.Fatalf("second LoadOrCreateDevice() error = %v", err)
	}
	if second.Id != first.Id || second.Name != first.Name || second.PublicKey != first.PublicKey || !second.CreatedAt.Equal(first.CreatedAt) || !second.UpdatedAt.Equal(first.UpdatedAt) {
		t.Fatalf("second = %#v, first = %#v", second, first)
	}
}

func TestLoadOrCreateDeviceRejectsMissingStateDir(t *testing.T) {
	_, err := LoadOrCreateDevice(DeviceOptions{})
	if err == nil {
		t.Fatal("LoadOrCreateDevice() error = nil, want error")
	}
}

func TestLoadOrCreateDeviceRejectsBadPrivateKey(t *testing.T) {
	stateDir := t.TempDir()
	device, err := LoadOrCreateDevice(DeviceOptions{StateDir: stateDir})
	if err != nil {
		t.Fatalf("LoadOrCreateDevice() error = %v", err)
	}
	path := filepath.Join(stateDir, "devices", device.Id, PrivateKeyFileName)
	if err := os.WriteFile(path, []byte("not pem"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	_, err = LoadOrCreateDevice(DeviceOptions{StateDir: stateDir})
	if err == nil {
		t.Fatal("LoadOrCreateDevice() error = nil, want error")
	}
}

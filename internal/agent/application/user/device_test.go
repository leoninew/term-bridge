package application

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCloudBindingSummaryPersistsWithDeviceIdentityWithoutTokens(t *testing.T) {
	stateDir := t.TempDir()
	createdAt := time.Date(2026, 7, 5, 10, 0, 0, 0, time.UTC)
	device, err := LoadOrCreateDevice(DeviceOptions{StateDir: stateDir, Now: func() time.Time { return createdAt }})
	if err != nil {
		t.Fatalf("LoadOrCreateDevice() error = %v", err)
	}
	connectedAt := time.Date(2026, 7, 5, 11, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC)
	summary := CloudBindingSummary{PublicUrl: "https://cloud.example.test/", DeviceId: device.Id, DeviceName: device.Name, ConnectedAt: connectedAt}

	if err := SaveCloudBindingSummary(stateDir, summary, updatedAt); err != nil {
		t.Fatalf("SaveCloudBindingSummary() error = %v", err)
	}

	loaded, err := LoadOrCreateDevice(DeviceOptions{StateDir: stateDir, Now: func() time.Time { return updatedAt.Add(time.Hour) }})
	if err != nil {
		t.Fatalf("LoadOrCreateDevice(reload) error = %v", err)
	}
	if loaded.Id != device.Id || loaded.Name != device.Name || loaded.PublicKey != device.PublicKey {
		t.Fatalf("device identity changed after cloud binding save: before=%#v after=%#v", device, loaded)
	}
	if loaded.CloudBinding == nil {
		t.Fatalf("cloud binding was not loaded with device identity")
	}
	if loaded.CloudBinding.PublicUrl != "https://cloud.example.test" || loaded.CloudBinding.DeviceId != device.Id || loaded.CloudBinding.DeviceName != device.Name || !loaded.CloudBinding.ConnectedAt.Equal(connectedAt) {
		t.Fatalf("cloud binding = %#v", loaded.CloudBinding)
	}

	data, err := os.ReadFile(filepath.Join(stateDir, DeviceIdentityFileName))
	if err != nil {
		t.Fatalf("read device identity file error = %v", err)
	}
	serialized := strings.ToLower(string(data))
	if strings.Contains(serialized, "access_token") || strings.Contains(serialized, "refresh_token") || strings.Contains(serialized, "cloud-token") || strings.Contains(serialized, "bearer") {
		t.Fatalf("device identity persisted token material: %s", string(data))
	}
	var persisted struct {
		CloudBinding CloudBindingSummary `json:"cloud_binding"`
	}
	if err := json.Unmarshal(data, &persisted); err != nil {
		t.Fatalf("decode device identity file: %v", err)
	}
	if persisted.CloudBinding.PublicUrl != "https://cloud.example.test" || !persisted.CloudBinding.ConnectedAt.Equal(connectedAt) {
		t.Fatalf("persisted cloud binding = %#v", persisted.CloudBinding)
	}
}

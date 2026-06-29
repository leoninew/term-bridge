package agent

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	DeviceSchemaVersion = 1
	DeviceFileName      = "device.json"
	PrivateKeyFileName  = "private_key.pem"
	PublicKeyFileName   = "public_key.pem"
)

type Device struct {
	SchemaVersion int       `json:"schema_version"`
	Id            string    `json:"id"`
	Name          string    `json:"name"`
	PublicKey     string    `json:"public_key"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
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
	deviceDir := filepath.Join(options.StateDir, "devices", safeDeviceSegment(deviceId))
	keys, err := loadOrCreateDeviceKeys(deviceDir)
	if err != nil {
		return Device{}, err
	}
	path := filepath.Join(deviceDir, DeviceFileName)
	device, err := readDevice(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return Device{}, err
		}
		device = Device{SchemaVersion: DeviceSchemaVersion, Id: deviceId, Name: deviceName, CreatedAt: now}
	}
	device.SchemaVersion = DeviceSchemaVersion
	device.Id = deviceId
	device.Name = deviceName
	device.PublicKey = keys.PublicKeyBase64
	if device.CreatedAt.IsZero() {
		device.CreatedAt = now
	}
	device.UpdatedAt = now
	if err := writeDevice(path, device); err != nil {
		return Device{}, err
	}
	return device, nil
}

type deviceKeys struct {
	PrivateKey      ed25519.PrivateKey
	PublicKey       ed25519.PublicKey
	PublicKeyBase64 string
}

func loadOrCreateDeviceKeys(deviceDir string) (deviceKeys, error) {
	privatePath := filepath.Join(deviceDir, PrivateKeyFileName)
	publicPath := filepath.Join(deviceDir, PublicKeyFileName)
	privateKey, err := readPrivateKey(privatePath)
	if err == nil {
		publicKey := privateKey.Public().(ed25519.PublicKey)
		if err := ensurePublicKeyFile(publicPath, publicKey); err != nil {
			return deviceKeys{}, err
		}
		return deviceKeys{PrivateKey: privateKey, PublicKey: publicKey, PublicKeyBase64: base64.StdEncoding.EncodeToString(publicKey)}, nil
	}
	if !os.IsNotExist(err) {
		return deviceKeys{}, err
	}
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return deviceKeys{}, fmt.Errorf("generate device key: %w", err)
	}
	if err := writePEMFile(privatePath, "ED25519 PRIVATE KEY", privateKey, 0o600); err != nil {
		return deviceKeys{}, err
	}
	if err := ensurePublicKeyFile(publicPath, publicKey); err != nil {
		return deviceKeys{}, err
	}
	return deviceKeys{PrivateKey: privateKey, PublicKey: publicKey, PublicKeyBase64: base64.StdEncoding.EncodeToString(publicKey)}, nil
}

func ensurePublicKeyFile(path string, publicKey ed25519.PublicKey) error {
	return writePEMFile(path, "ED25519 PUBLIC KEY", publicKey, 0o644)
}

func LoadDevicePrivateKey(stateDir string, deviceId string) (ed25519.PrivateKey, error) {
	return readPrivateKey(filepath.Join(stateDir, "devices", safeDeviceSegment(deviceId), PrivateKeyFileName))
}

func LoadDevicePublicKey(stateDir string, deviceId string) (ed25519.PublicKey, error) {
	privateKey, err := LoadDevicePrivateKey(stateDir, deviceId)
	if err != nil {
		return nil, err
	}
	return privateKey.Public().(ed25519.PublicKey), nil
}

func readPrivateKey(path string) (ed25519.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(data)
	if block == nil || block.Type != "ED25519 PRIVATE KEY" {
		return nil, fmt.Errorf("read device private key: invalid PEM")
	}
	if len(block.Bytes) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("read device private key: invalid length")
	}
	return ed25519.PrivateKey(append([]byte(nil), block.Bytes...)), nil
}

func writePEMFile(path string, blockType string, data []byte, perm os.FileMode) error {
	encoded := pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: data})
	return writeFileAtomic(path, encoded, perm)
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
	if device.SchemaVersion != 0 && device.SchemaVersion != DeviceSchemaVersion {
		return Device{}, fmt.Errorf("read device identity: unsupported schema_version %d", device.SchemaVersion)
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
	data, err := json.MarshalIndent(device, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return writeFileAtomic(path, data, 0o644)
}

func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return nil
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

package application

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	DeviceSchemaVersion    = 1
	DeviceIdentityFileName = "device.json"
	PrivateKeyFileName     = "private_key.pem"
	PublicKeyFileName      = "public_key.pem"
)

type Device struct {
	SchemaVersion int                  `json:"schema_version"`
	Id            string               `json:"id"`
	Name          string               `json:"name"`
	PublicKey     string               `json:"public_key"`
	CloudBinding  *CloudBindingSummary `json:"cloud_binding,omitempty"`
	CreatedAt     time.Time            `json:"created_at"`
	UpdatedAt     time.Time            `json:"updated_at"`
}

type CloudBindingSummary struct {
	PublicUrl   string    `json:"public_url"`
	DeviceId    string    `json:"device_id"`
	DeviceName  string    `json:"device_name"`
	ConnectedAt time.Time `json:"connected_at"`
}

type DeviceOptions struct {
	StateDir string
	Now      func() time.Time
}

type deviceIdentity struct {
	SchemaVersion int                  `json:"schema_version"`
	Id            string               `json:"id"`
	Name          string               `json:"name"`
	CloudBinding  *CloudBindingSummary `json:"cloud_binding,omitempty"`
	CreatedAt     time.Time            `json:"created_at"`
	UpdatedAt     time.Time            `json:"updated_at"`
}

func LoadOrCreateDevice(options DeviceOptions) (Device, error) {
	if strings.TrimSpace(options.StateDir) == "" {
		return Device{}, fmt.Errorf("state dir is required")
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	identity, err := loadOrCreateDeviceIdentity(options.StateDir, options.Now().UTC())
	if err != nil {
		return Device{}, err
	}
	keys, err := loadOrCreateDeviceKeys(options.StateDir)
	if err != nil {
		return Device{}, err
	}
	return Device{SchemaVersion: DeviceSchemaVersion, Id: identity.Id, Name: identity.Name, PublicKey: keys.PublicKeyBase64, CloudBinding: cloneCloudBindingSummary(identity.CloudBinding), CreatedAt: identity.CreatedAt, UpdatedAt: identity.UpdatedAt}, nil
}

func loadOrCreateDeviceIdentity(stateDir string, now time.Time) (deviceIdentity, error) {
	path := filepath.Join(stateDir, DeviceIdentityFileName)
	identity, err := readDeviceIdentity(path)
	if err == nil {
		return identity, nil
	}
	if !os.IsNotExist(err) {
		return deviceIdentity{}, err
	}
	id, err := randomDeviceId()
	if err != nil {
		return deviceIdentity{}, err
	}
	identity = deviceIdentity{SchemaVersion: DeviceSchemaVersion, Id: id, Name: defaultDeviceName(), CreatedAt: now, UpdatedAt: now}
	if err := writeDeviceIdentity(path, identity); err != nil {
		return deviceIdentity{}, err
	}
	return identity, nil
}

func readDeviceIdentity(path string) (deviceIdentity, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return deviceIdentity{}, err
	}
	var identity deviceIdentity
	if err := json.Unmarshal(data, &identity); err != nil {
		return deviceIdentity{}, fmt.Errorf("read device identity: %w", err)
	}
	if identity.SchemaVersion != DeviceSchemaVersion {
		return deviceIdentity{}, fmt.Errorf("read device identity: unsupported schema version %d", identity.SchemaVersion)
	}
	if strings.TrimSpace(identity.Id) == "" {
		return deviceIdentity{}, fmt.Errorf("read device identity: id is required")
	}
	if strings.TrimSpace(identity.Name) == "" {
		return deviceIdentity{}, fmt.Errorf("read device identity: name is required")
	}
	identity.CloudBinding = cloneCloudBindingSummary(identity.CloudBinding)
	return identity, nil
}

func writeDeviceIdentity(path string, identity deviceIdentity) error {
	identity.CloudBinding = cloneCloudBindingSummary(identity.CloudBinding)
	data, err := json.MarshalIndent(identity, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return writeFileAtomic(path, data, 0o600)
}

func LoadCloudBindingSummary(stateDir string) (*CloudBindingSummary, error) {
	if strings.TrimSpace(stateDir) == "" {
		return nil, fmt.Errorf("state dir is required")
	}
	identity, err := readDeviceIdentity(filepath.Join(stateDir, DeviceIdentityFileName))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return cloneCloudBindingSummary(identity.CloudBinding), nil
}

func SaveCloudBindingSummary(stateDir string, summary CloudBindingSummary, now time.Time) error {
	if strings.TrimSpace(stateDir) == "" {
		return fmt.Errorf("state dir is required")
	}
	if strings.TrimSpace(summary.PublicUrl) == "" {
		return fmt.Errorf("cloud binding public URL is required")
	}
	if strings.TrimSpace(summary.DeviceId) == "" {
		return fmt.Errorf("cloud binding device id is required")
	}
	if strings.TrimSpace(summary.DeviceName) == "" {
		return fmt.Errorf("cloud binding device name is required")
	}
	if summary.ConnectedAt.IsZero() {
		return fmt.Errorf("cloud binding connected time is required")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	path := filepath.Join(stateDir, DeviceIdentityFileName)
	identity, err := readDeviceIdentity(path)
	if err != nil {
		return err
	}
	summary.PublicUrl = strings.TrimRight(strings.TrimSpace(summary.PublicUrl), "/")
	summary.DeviceId = strings.TrimSpace(summary.DeviceId)
	summary.DeviceName = strings.TrimSpace(summary.DeviceName)
	summary.ConnectedAt = summary.ConnectedAt.UTC()
	identity.CloudBinding = &summary
	identity.UpdatedAt = now.UTC()
	return writeDeviceIdentity(path, identity)
}

func ClearCloudBindingSummary(stateDir string, now time.Time) error {
	if strings.TrimSpace(stateDir) == "" {
		return fmt.Errorf("state dir is required")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	path := filepath.Join(stateDir, DeviceIdentityFileName)
	identity, err := readDeviceIdentity(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if identity.CloudBinding == nil {
		return nil
	}
	identity.CloudBinding = nil
	identity.UpdatedAt = now.UTC()
	return writeDeviceIdentity(path, identity)
}

func cloneCloudBindingSummary(summary *CloudBindingSummary) *CloudBindingSummary {
	if summary == nil {
		return nil
	}
	clone := *summary
	clone.PublicUrl = strings.TrimRight(strings.TrimSpace(clone.PublicUrl), "/")
	clone.DeviceId = strings.TrimSpace(clone.DeviceId)
	clone.DeviceName = strings.TrimSpace(clone.DeviceName)
	clone.ConnectedAt = clone.ConnectedAt.UTC()
	return &clone
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
	return readPrivateKey(filepath.Join(stateDir, PrivateKeyFileName))
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

func randomDeviceId() (string, error) {
	data := make([]byte, 16)
	if _, err := rand.Read(data); err != nil {
		return "", fmt.Errorf("generate device id: %w", err)
	}
	return hex.EncodeToString(data), nil
}

func defaultDeviceName() string {
	hostname, err := os.Hostname()
	if err != nil || strings.TrimSpace(hostname) == "" {
		hostname = "termbridge-device"
	}
	hostname = sanitizeName(hostname)
	if hostname == "" {
		return "termbridge-device"
	}
	return hostname
}

func sanitizeName(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, " ", "-")
	value = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.' {
			return r
		}
		return -1
	}, value)
	return strings.Trim(value, "-_.")
}

package authapp

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	CloudBindingAttemptSchemaVersion = 1
	CloudBindingAttemptTTL           = 10 * time.Minute
)

type CloudBindingAttemptStore struct {
	stateDir string
	path     string
	now      func() time.Time
}

type cloudBindingAttemptFile struct {
	SchemaVersion int                         `json:"schema_version"`
	Attempts      []cloudBindingAttemptRecord `json:"attempts"`
}

type cloudBindingAttemptRecord struct {
	StateHash        string     `json:"state_hash"`
	CallbackURL      string     `json:"callback_url"`
	PostAuthRedirect string     `json:"post_auth_redirect"`
	GateURL          string     `json:"gate_url"`
	DeviceID         string     `json:"device_id"`
	DeviceName       string     `json:"device_name"`
	CreatedAt        time.Time  `json:"created_at"`
	ExpiresAt        time.Time  `json:"expires_at"`
	CompletedAt      *time.Time `json:"completed_at,omitempty"`
}

type CloudBindingAttemptOptions struct {
	CallbackURL      string
	PostAuthRedirect string
	GateURL          string
	DeviceID         string
	DeviceName       string
	TTL              time.Duration
}

type CloudBindingAttempt struct {
	CallbackURL      string
	PostAuthRedirect string
	GateURL          string
	DeviceID         string
	DeviceName       string
}

func NewCloudBindingAttemptStore(stateDir string) *CloudBindingAttemptStore {
	return &CloudBindingAttemptStore{stateDir: stateDir, path: filepath.Join(stateDir, "auth", "cloud-binding-attempts.json"), now: time.Now}
}

func (s *CloudBindingAttemptStore) Create(callbackURL string, ttl time.Duration) (string, error) {
	return s.CreateWithOptions(CloudBindingAttemptOptions{CallbackURL: callbackURL, TTL: ttl})
}

func (s *CloudBindingAttemptStore) CreateWithOptions(options CloudBindingAttemptOptions) (string, error) {
	if s == nil {
		return "", fmt.Errorf("cloud binding attempt store is nil")
	}
	callbackURL := strings.TrimSpace(options.CallbackURL)
	if callbackURL == "" {
		return "", fmt.Errorf("callback URL is required")
	}
	ttl := options.TTL
	if ttl <= 0 {
		ttl = CloudBindingAttemptTTL
	}
	state, err := randomCloudBindingState()
	if err != nil {
		return "", err
	}
	now := s.now().UTC()
	file, err := s.load()
	if err != nil {
		return "", err
	}
	file.SchemaVersion = CloudBindingAttemptSchemaVersion
	file.Attempts = append(activeCloudBindingAttempts(file.Attempts, now), cloudBindingAttemptRecord{StateHash: hashCloudBindingState(state), CallbackURL: callbackURL, PostAuthRedirect: cleanLocalRedirect(options.PostAuthRedirect), GateURL: options.GateURL, DeviceID: options.DeviceID, DeviceName: options.DeviceName, CreatedAt: now, ExpiresAt: now.Add(ttl)})
	if err := s.save(file); err != nil {
		return "", err
	}
	return state, nil
}

func (s *CloudBindingAttemptStore) Complete(state string) (bool, error) {
	_, ok, err := s.CompleteAttempt(state)
	return ok, err
}

func (s *CloudBindingAttemptStore) CompleteAttempt(state string) (CloudBindingAttempt, bool, error) {
	if s == nil {
		return CloudBindingAttempt{}, false, fmt.Errorf("cloud binding attempt store is nil")
	}
	state = strings.TrimSpace(state)
	if state == "" {
		return CloudBindingAttempt{}, false, nil
	}
	file, err := s.load()
	if err != nil {
		return CloudBindingAttempt{}, false, err
	}
	now := s.now().UTC()
	stateHash := hashCloudBindingState(state)
	var attempt CloudBindingAttempt
	completed := false
	for i := range file.Attempts {
		record := &file.Attempts[i]
		if record.StateHash != stateHash {
			continue
		}
		if record.CompletedAt != nil || !record.ExpiresAt.After(now) {
			return CloudBindingAttempt{}, false, nil
		}
		record.CompletedAt = &now
		attempt = CloudBindingAttempt{CallbackURL: record.CallbackURL, PostAuthRedirect: record.PostAuthRedirect, GateURL: record.GateURL, DeviceID: record.DeviceID, DeviceName: record.DeviceName}
		completed = true
		break
	}
	if !completed {
		return CloudBindingAttempt{}, false, nil
	}
	file.Attempts = activeCloudBindingAttempts(file.Attempts, now)
	if err := s.save(file); err != nil {
		return CloudBindingAttempt{}, false, err
	}
	return attempt, true, nil
}

func (s *CloudBindingAttemptStore) load() (cloudBindingAttemptFile, error) {
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return cloudBindingAttemptFile{SchemaVersion: CloudBindingAttemptSchemaVersion}, nil
	}
	if err != nil {
		return cloudBindingAttemptFile{}, err
	}
	var file cloudBindingAttemptFile
	if err := json.Unmarshal(data, &file); err != nil {
		return cloudBindingAttemptFile{}, fmt.Errorf("read cloud binding attempts: %w", err)
	}
	if file.SchemaVersion != CloudBindingAttemptSchemaVersion {
		return cloudBindingAttemptFile{}, fmt.Errorf("read cloud binding attempts: unsupported schema_version %d", file.SchemaVersion)
	}
	return file, nil
}

func (s *CloudBindingAttemptStore) save(file cloudBindingAttemptFile) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".tmp-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return nil
}

func (s *CloudBindingAttemptStore) SaveConnection(connection CloudConnection) error {
	if s == nil {
		return fmt.Errorf("cloud binding attempt store is nil")
	}
	if connection.ConnectedAt.IsZero() {
		connection.ConnectedAt = s.now().UTC()
	}
	path := filepath.Join(s.stateDir, "cloud", "connection.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(connection, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
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

type CloudConnection struct {
	SchemaVersion int       `json:"schema_version"`
	GateURL       string    `json:"gate_url"`
	DeviceID      string    `json:"device_id"`
	DeviceName    string    `json:"device_name"`
	ConnectedAt   time.Time `json:"connected_at"`
}

func activeCloudBindingAttempts(attempts []cloudBindingAttemptRecord, now time.Time) []cloudBindingAttemptRecord {
	active := make([]cloudBindingAttemptRecord, 0, len(attempts))
	for _, attempt := range attempts {
		if attempt.CompletedAt == nil && attempt.ExpiresAt.After(now) {
			active = append(active, attempt)
		}
	}
	return active
}

func cleanLocalRedirect(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || !strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") {
		return "/dashboard"
	}
	return value
}

func randomCloudBindingState() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func hashCloudBindingState(state string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(state)))
	return hex.EncodeToString(sum[:])
}

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
	CloudOAuthAttemptSchemaVersion = 1
	CloudOAuthAttemptTTL           = 10 * time.Minute
)

type CloudOAuthAttemptStore struct {
	stateDir string
	path     string
	now      func() time.Time
}

type cloudOAuthAttemptFile struct {
	SchemaVersion int                       `json:"schema_version"`
	Attempts      []cloudOAuthAttemptRecord `json:"attempts"`
}

type cloudOAuthAttemptRecord struct {
	StateHash        string     `json:"state_hash"`
	CallbackURL      string     `json:"callback_url"`
	PostAuthRedirect string     `json:"post_auth_redirect"`
	GateURL          string     `json:"gate_url"`
	DeviceId         string     `json:"device_id"`
	DeviceName       string     `json:"device_name"`
	CreatedAt        time.Time  `json:"created_at"`
	ExpiresAt        time.Time  `json:"expires_at"`
	CompletedAt      *time.Time `json:"completed_at,omitempty"`
}

type CloudOAuthAttemptOptions struct {
	CallbackURL      string
	PostAuthRedirect string
	GateURL          string
	DeviceId         string
	DeviceName       string
	TTL              time.Duration
}

type CloudOAuthAttempt struct {
	CallbackURL      string
	PostAuthRedirect string
	GateURL          string
	DeviceId         string
	DeviceName       string
}

func NewCloudOAuthAttemptStore(stateDir string) *CloudOAuthAttemptStore {
	return &CloudOAuthAttemptStore{stateDir: stateDir, path: filepath.Join(stateDir, "auth", "cloud-oauth-attempts.json"), now: time.Now}
}

func (s *CloudOAuthAttemptStore) Create(callbackURL string, ttl time.Duration) (string, error) {
	return s.CreateWithOptions(CloudOAuthAttemptOptions{CallbackURL: callbackURL, TTL: ttl})
}

func (s *CloudOAuthAttemptStore) CreateWithOptions(options CloudOAuthAttemptOptions) (string, error) {
	if s == nil {
		return "", fmt.Errorf("cloud oauth attempt store is nil")
	}
	callbackURL := strings.TrimSpace(options.CallbackURL)
	if callbackURL == "" {
		return "", fmt.Errorf("callback URL is required")
	}
	ttl := options.TTL
	if ttl <= 0 {
		ttl = CloudOAuthAttemptTTL
	}
	state, err := randomCloudOAuthState()
	if err != nil {
		return "", err
	}
	now := s.now().UTC()
	file, err := s.load()
	if err != nil {
		return "", err
	}
	file.SchemaVersion = CloudOAuthAttemptSchemaVersion
	file.Attempts = append(activeCloudOAuthAttempts(file.Attempts, now), cloudOAuthAttemptRecord{StateHash: hashCloudOAuthState(state), CallbackURL: callbackURL, PostAuthRedirect: cleanLocalRedirect(options.PostAuthRedirect), GateURL: options.GateURL, DeviceId: options.DeviceId, DeviceName: options.DeviceName, CreatedAt: now, ExpiresAt: now.Add(ttl)})
	if err := s.save(file); err != nil {
		return "", err
	}
	return state, nil
}

func (s *CloudOAuthAttemptStore) Complete(state string) (bool, error) {
	_, ok, err := s.CompleteAttempt(state)
	return ok, err
}

func (s *CloudOAuthAttemptStore) CompleteAttempt(state string) (CloudOAuthAttempt, bool, error) {
	if s == nil {
		return CloudOAuthAttempt{}, false, fmt.Errorf("cloud oauth attempt store is nil")
	}
	state = strings.TrimSpace(state)
	if state == "" {
		return CloudOAuthAttempt{}, false, nil
	}
	file, err := s.load()
	if err != nil {
		return CloudOAuthAttempt{}, false, err
	}
	now := s.now().UTC()
	stateHash := hashCloudOAuthState(state)
	var attempt CloudOAuthAttempt
	completed := false
	for i := range file.Attempts {
		record := &file.Attempts[i]
		if record.StateHash != stateHash {
			continue
		}
		if record.CompletedAt != nil || !record.ExpiresAt.After(now) {
			return CloudOAuthAttempt{}, false, nil
		}
		record.CompletedAt = &now
		attempt = CloudOAuthAttempt{CallbackURL: record.CallbackURL, PostAuthRedirect: record.PostAuthRedirect, GateURL: record.GateURL, DeviceId: record.DeviceId, DeviceName: record.DeviceName}
		completed = true
		break
	}
	if !completed {
		return CloudOAuthAttempt{}, false, nil
	}
	file.Attempts = activeCloudOAuthAttempts(file.Attempts, now)
	if err := s.save(file); err != nil {
		return CloudOAuthAttempt{}, false, err
	}
	return attempt, true, nil
}

func (s *CloudOAuthAttemptStore) load() (cloudOAuthAttemptFile, error) {
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return cloudOAuthAttemptFile{SchemaVersion: CloudOAuthAttemptSchemaVersion}, nil
	}
	if err != nil {
		return cloudOAuthAttemptFile{}, err
	}
	var file cloudOAuthAttemptFile
	if err := json.Unmarshal(data, &file); err != nil {
		return cloudOAuthAttemptFile{}, fmt.Errorf("read cloud oauth attempts: %w", err)
	}
	if file.SchemaVersion != CloudOAuthAttemptSchemaVersion {
		return cloudOAuthAttemptFile{}, fmt.Errorf("read cloud oauth attempts: unsupported schema_version %d", file.SchemaVersion)
	}
	return file, nil
}

func (s *CloudOAuthAttemptStore) save(file cloudOAuthAttemptFile) error {
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

func activeCloudOAuthAttempts(attempts []cloudOAuthAttemptRecord, now time.Time) []cloudOAuthAttemptRecord {
	active := make([]cloudOAuthAttemptRecord, 0, len(attempts))
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

func randomCloudOAuthState() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func hashCloudOAuthState(state string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(state)))
	return hex.EncodeToString(sum[:])
}

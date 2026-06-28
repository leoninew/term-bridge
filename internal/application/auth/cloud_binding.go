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
	path string
	now  func() time.Time
}

type cloudBindingAttemptFile struct {
	SchemaVersion int                         `json:"schema_version"`
	Attempts      []cloudBindingAttemptRecord `json:"attempts"`
}

type cloudBindingAttemptRecord struct {
	StateHash   string     `json:"state_hash"`
	CallbackURL string     `json:"callback_url"`
	CreatedAt   time.Time  `json:"created_at"`
	ExpiresAt   time.Time  `json:"expires_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

func NewCloudBindingAttemptStore(stateDir string) *CloudBindingAttemptStore {
	return &CloudBindingAttemptStore{path: filepath.Join(stateDir, "auth", "cloud-binding-attempts.json"), now: time.Now}
}

func (s *CloudBindingAttemptStore) Create(callbackURL string, ttl time.Duration) (string, error) {
	if s == nil {
		return "", fmt.Errorf("cloud binding attempt store is nil")
	}
	callbackURL = strings.TrimSpace(callbackURL)
	if callbackURL == "" {
		return "", fmt.Errorf("callback URL is required")
	}
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
	file.Attempts = append(activeCloudBindingAttempts(file.Attempts, now), cloudBindingAttemptRecord{StateHash: hashCloudBindingState(state), CallbackURL: callbackURL, CreatedAt: now, ExpiresAt: now.Add(ttl)})
	if err := s.save(file); err != nil {
		return "", err
	}
	return state, nil
}

func (s *CloudBindingAttemptStore) Complete(state string) (bool, error) {
	if s == nil {
		return false, fmt.Errorf("cloud binding attempt store is nil")
	}
	state = strings.TrimSpace(state)
	if state == "" {
		return false, nil
	}
	file, err := s.load()
	if err != nil {
		return false, err
	}
	now := s.now().UTC()
	stateHash := hashCloudBindingState(state)
	completed := false
	for i := range file.Attempts {
		record := &file.Attempts[i]
		if record.StateHash != stateHash {
			continue
		}
		if record.CompletedAt != nil || !record.ExpiresAt.After(now) {
			return false, nil
		}
		record.CompletedAt = &now
		completed = true
		break
	}
	if !completed {
		return false, nil
	}
	file.Attempts = activeCloudBindingAttempts(file.Attempts, now)
	if err := s.save(file); err != nil {
		return false, err
	}
	return true, nil
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

func activeCloudBindingAttempts(attempts []cloudBindingAttemptRecord, now time.Time) []cloudBindingAttemptRecord {
	active := make([]cloudBindingAttemptRecord, 0, len(attempts))
	for _, attempt := range attempts {
		if attempt.CompletedAt == nil && attempt.ExpiresAt.After(now) {
			active = append(active, attempt)
		}
	}
	return active
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

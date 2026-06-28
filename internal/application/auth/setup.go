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
	SetupTokenSchemaVersion = 1
	SetupTokenTTL           = 10 * time.Minute
)

type SetupTokenStore struct {
	path string
	now  func() time.Time
}

type setupTokenFile struct {
	SchemaVersion int                `json:"schema_version"`
	Tokens        []setupTokenRecord `json:"tokens"`
}

type setupTokenRecord struct {
	Hash      string     `json:"hash"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt time.Time  `json:"expires_at"`
	UsedAt    *time.Time `json:"used_at,omitempty"`
}

func NewSetupTokenStore(stateDir string) *SetupTokenStore {
	return &SetupTokenStore{path: filepath.Join(stateDir, "auth", "setup-tokens.json"), now: time.Now}
}

func (s *SetupTokenStore) Create(ttl time.Duration) (string, error) {
	if s == nil {
		return "", fmt.Errorf("setup token store is nil")
	}
	if ttl <= 0 {
		ttl = SetupTokenTTL
	}
	token, err := randomSetupToken()
	if err != nil {
		return "", err
	}
	now := s.now().UTC()
	file, err := s.load()
	if err != nil {
		return "", err
	}
	file.SchemaVersion = SetupTokenSchemaVersion
	file.Tokens = append(activeSetupTokens(file.Tokens, now), setupTokenRecord{Hash: hashSetupToken(token), CreatedAt: now, ExpiresAt: now.Add(ttl)})
	if err := s.save(file); err != nil {
		return "", err
	}
	return token, nil
}

func (s *SetupTokenStore) Use(token string) (bool, error) {
	if s == nil {
		return false, fmt.Errorf("setup token store is nil")
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return false, nil
	}
	file, err := s.load()
	if err != nil {
		return false, err
	}
	now := s.now().UTC()
	hash := hashSetupToken(token)
	used := false
	for i := range file.Tokens {
		record := &file.Tokens[i]
		if record.Hash != hash {
			continue
		}
		if record.UsedAt != nil || !record.ExpiresAt.After(now) {
			return false, nil
		}
		record.UsedAt = &now
		used = true
		break
	}
	if !used {
		return false, nil
	}
	file.Tokens = activeSetupTokens(file.Tokens, now)
	if err := s.save(file); err != nil {
		return false, err
	}
	return true, nil
}

func (s *SetupTokenStore) Available() (bool, error) {
	if s == nil {
		return false, nil
	}
	file, err := s.load()
	if err != nil {
		return false, err
	}
	now := s.now().UTC()
	for _, record := range file.Tokens {
		if record.UsedAt == nil && record.ExpiresAt.After(now) {
			return true, nil
		}
	}
	return false, nil
}

func (s *SetupTokenStore) load() (setupTokenFile, error) {
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return setupTokenFile{SchemaVersion: SetupTokenSchemaVersion}, nil
	}
	if err != nil {
		return setupTokenFile{}, err
	}
	var file setupTokenFile
	if err := json.Unmarshal(data, &file); err != nil {
		return setupTokenFile{}, fmt.Errorf("read setup tokens: %w", err)
	}
	if file.SchemaVersion != SetupTokenSchemaVersion {
		return setupTokenFile{}, fmt.Errorf("read setup tokens: unsupported schema_version %d", file.SchemaVersion)
	}
	return file, nil
}

func (s *SetupTokenStore) save(file setupTokenFile) error {
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

func activeSetupTokens(tokens []setupTokenRecord, now time.Time) []setupTokenRecord {
	active := make([]setupTokenRecord, 0, len(tokens))
	for _, token := range tokens {
		if token.UsedAt == nil && token.ExpiresAt.After(now) {
			active = append(active, token)
		}
	}
	return active
}

func randomSetupToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func hashSetupToken(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}

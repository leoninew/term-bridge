package api

import (
	"crypto/rand"
	"encoding/base64"
	"strings"
	"sync"
)

type testCloudOAuthAttemptStore struct {
	mu       sync.Mutex
	attempts map[string]CloudOAuthAttempt
}

func newTestCloudOAuthAttemptStore(stateDir string) CloudOAuthAttemptRepository {
	_ = stateDir
	return &testCloudOAuthAttemptStore{attempts: make(map[string]CloudOAuthAttempt)}
}

func (s *testCloudOAuthAttemptStore) CreateWithOptions(options CloudOAuthAttemptOptions) (string, error) {
	state, err := randomTestCloudOAuthState()
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.attempts[state] = CloudOAuthAttempt{CallbackURL: strings.TrimSpace(options.CallbackURL), PostAuthRedirect: options.PostAuthRedirect, GateURL: options.GateURL, DeviceId: options.DeviceId, DeviceName: options.DeviceName}
	return state, nil
}

func (s *testCloudOAuthAttemptStore) CompleteAttempt(state string) (CloudOAuthAttempt, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	attempt, ok := s.attempts[strings.TrimSpace(state)]
	if !ok {
		return CloudOAuthAttempt{}, false, nil
	}
	delete(s.attempts, state)
	return attempt, true, nil
}

func randomTestCloudOAuthState() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

package authapp

import (
	"testing"
	"time"
)

func TestCloudOAuthAttemptStoreCreateAndComplete(t *testing.T) {
	store := NewCloudOAuthAttemptStore(t.TempDir())
	now := time.Date(2026, 6, 29, 10, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }

	state, err := store.CreateWithOptions(CloudOAuthAttemptOptions{CallbackURL: "http://localhost:9031/cloud/oauth/callback", PostAuthRedirect: "/dashboard", GateURL: "https://cloud.example.test", DeviceId: "dev-1", DeviceName: "local-device", TTL: CloudOAuthAttemptTTL})
	if err != nil {
		t.Fatalf("CreateWithOptions() error = %v", err)
	}
	if state == "" {
		t.Fatal("state is empty")
	}

	attempt, ok, err := store.CompleteAttempt(state)
	if err != nil {
		t.Fatalf("CompleteAttempt() error = %v", err)
	}
	if !ok {
		t.Fatal("CompleteAttempt() ok = false")
	}
	if attempt.CallbackURL != "http://localhost:9031/cloud/oauth/callback" || attempt.PostAuthRedirect != "/dashboard" || attempt.GateURL != "https://cloud.example.test" || attempt.DeviceId != "dev-1" || attempt.DeviceName != "local-device" {
		t.Fatalf("attempt = %#v", attempt)
	}
}

func TestCloudOAuthAttemptStoreRejectsStateReuse(t *testing.T) {
	store := NewCloudOAuthAttemptStore(t.TempDir())
	state, err := store.Create("http://localhost:9031/cloud/oauth/callback", CloudOAuthAttemptTTL)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, ok, err := store.CompleteAttempt(state); err != nil || !ok {
		t.Fatalf("first CompleteAttempt() = ok %v, err %v", ok, err)
	}
	if _, ok, err := store.CompleteAttempt(state); err != nil {
		t.Fatalf("second CompleteAttempt() error = %v", err)
	} else if ok {
		t.Fatal("second CompleteAttempt() ok = true")
	}
}

func TestCloudOAuthAttemptStoreRejectsExpiredState(t *testing.T) {
	store := NewCloudOAuthAttemptStore(t.TempDir())
	now := time.Date(2026, 6, 29, 10, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	state, err := store.Create("http://localhost:9031/cloud/oauth/callback", time.Minute)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	store.now = func() time.Time { return now.Add(2 * time.Minute) }
	if _, ok, err := store.CompleteAttempt(state); err != nil {
		t.Fatalf("CompleteAttempt() error = %v", err)
	} else if ok {
		t.Fatal("CompleteAttempt() ok = true")
	}
}

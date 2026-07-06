package api

import (
	"context"
	"crypto/ed25519"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	clouddevice "termbridge/internal/cloud/repository/user/device"
	sharedauth "termbridge/internal/shared/common/auth"
	"termbridge/internal/shared/dto/protocol/tunnel"
)

func TestCurrentDeviceReportBindsDeviceToCloudUserAndTunnelUsesPublicKey(t *testing.T) {
	handler := newCloudHandlerForTest(t)
	token := cloudUserToken(t, handler.authService, "user-1", "user-1@example.test")
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	localDevice := Device{Id: "dev-1", Name: "local-device", PublicKey: base64.StdEncoding.EncodeToString(publicKey)}

	reportRequest := httptest.NewRequest(http.MethodPost, "/cloud-api/devices/current", strings.NewReader(`{"id":"`+localDevice.Id+`","name":"`+localDevice.Name+`","public_key":"`+localDevice.PublicKey+`"}`))
	reportRequest.Header.Set("Authorization", "Bearer "+token)
	reportResponse := httptest.NewRecorder()
	handler.ServeHTTP(reportResponse, reportRequest)
	if reportResponse.Code != http.StatusOK {
		t.Fatalf("current device report status = %d; body=%s", reportResponse.Code, reportResponse.Body.String())
	}

	devicesRequest := httptest.NewRequest(http.MethodGet, "/cloud-api/devices", nil)
	devicesRequest.Header.Set("Authorization", "Bearer "+token)
	devicesResponse := httptest.NewRecorder()
	handler.ServeHTTP(devicesResponse, devicesRequest)
	if devicesResponse.Code != http.StatusOK {
		t.Fatalf("devices status = %d; body=%s", devicesResponse.Code, devicesResponse.Body.String())
	}
	var devices ListDevicesResp
	if err := json.Unmarshal(devicesResponse.Body.Bytes(), &devices); err != nil {
		t.Fatalf("decode devices: %v", err)
	}
	if len(devices.Items) != 1 || devices.Items[0].Id != localDevice.Id || devices.Items[0].Online {
		t.Fatalf("devices = %#v", devices)
	}
	storedPublicKey, err := handler.config.DeviceRepository.PublicKey(context.Background(), localDevice.Id)
	if err != nil {
		t.Fatalf("PublicKey(%s) error = %v", localDevice.Id, err)
	}
	if storedPublicKey != localDevice.PublicKey {
		t.Fatalf("PublicKey(%s) = %q", localDevice.Id, storedPublicKey)
	}
	handler.config.AgentTunnelAudience = "test-audience"
	tunnelHeader, err := tunnel.SignedTunnelHeader(http.MethodGet, "/cloud-api/agent/tunnel", "test-audience", localDevice.Id, privateKey, time.Now(), "test-nonce")
	if err != nil {
		t.Fatalf("SignedTunnelHeader() error = %v", err)
	}
	tunnelRequest := httptest.NewRequest(http.MethodGet, "/cloud-api/agent/tunnel", nil)
	tunnelRequest.Header = tunnelHeader
	verifiedDeviceId, ok := handler.verifyAgentTunnelRequest(tunnelRequest)
	if !ok || verifiedDeviceId != localDevice.Id {
		t.Fatalf("verifyAgentTunnelRequest() = %q, %v; want %q, true", verifiedDeviceId, ok, localDevice.Id)
	}
}

func TestCurrentDeviceReportRejectsUnauthenticatedRequest(t *testing.T) {
	handler := newCloudHandlerForTest(t)
	request := httptest.NewRequest(http.MethodPost, "/cloud-api/devices/current", strings.NewReader(`{"id":"dev-1","name":"local-device","public_key":"public-key"}`))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	assertAPIError(t, response, http.StatusUnauthorized, errorCodeUnauthorized)
}

func TestDevicesFiltersByCloudUserAndDeleteDisconnectsRoute(t *testing.T) {
	handler := newCloudHandlerForTest(t)
	repo := handler.config.DeviceRepository
	ctx := context.Background()
	if err := repo.UpsertDeviceBinding(ctx, "user-1", Device{Id: "dev-1", Name: "owned", PublicKey: base64.StdEncoding.EncodeToString([]byte("12345678901234567890123456789012"))}); err != nil {
		t.Fatalf("UpsertDeviceBinding(user-1) error = %v", err)
	}
	if err := repo.UpsertDeviceBinding(ctx, "user-2", Device{Id: "dev-2", Name: "other", PublicKey: base64.StdEncoding.EncodeToString([]byte("abcdefghijklmnopqrstuvwxyz123456"))}); err != nil {
		t.Fatalf("UpsertDeviceBinding(user-2) error = %v", err)
	}
	handler.registry.Register("dev-1", "owned", time.Now().UTC())
	handler.setRoute("dev-1", newAgentRoute("dev-1", nil))
	token := cloudUserToken(t, handler.authService, "user-1", "user-1@example.test")

	devicesRequest := httptest.NewRequest(http.MethodGet, "/cloud-api/devices", nil)
	devicesRequest.Header.Set("Authorization", "Bearer "+token)
	devicesResponse := httptest.NewRecorder()
	handler.ServeHTTP(devicesResponse, devicesRequest)
	if devicesResponse.Code != http.StatusOK {
		t.Fatalf("devices status = %d; body=%s", devicesResponse.Code, devicesResponse.Body.String())
	}
	var devices ListDevicesResp
	if err := json.Unmarshal(devicesResponse.Body.Bytes(), &devices); err != nil {
		t.Fatalf("decode devices: %v", err)
	}
	if len(devices.Items) != 1 || devices.Items[0].Id != "dev-1" || !devices.Items[0].Online {
		t.Fatalf("devices = %#v", devices)
	}

	deleteRequest := httptest.NewRequest(http.MethodDelete, "/cloud-api/devices/dev-1", nil)
	deleteRequest.Header.Set("Authorization", "Bearer "+token)
	deleteResponse := httptest.NewRecorder()
	handler.ServeHTTP(deleteResponse, deleteRequest)
	if deleteResponse.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d; body=%s", deleteResponse.Code, deleteResponse.Body.String())
	}
	if route := handler.routeFor("dev-1"); route != nil {
		t.Fatalf("route after delete = %#v, want nil", route)
	}
	owns, err := repo.UserOwnsDevice(ctx, "user-1", "dev-1")
	if err != nil {
		t.Fatalf("UserOwnsDevice(after delete) error = %v", err)
	}
	if owns {
		t.Fatal("user-1 still owns dev-1 after delete")
	}
}

func newCloudHandlerForTest(t *testing.T) *Handler {
	t.Helper()
	db := newCloudTestDB(t)
	deviceRepo := newCloudDeviceRepository(db)
	insertCloudUser(t, db, "user-1", "user-1@example.test")
	insertCloudUser(t, db, "user-2", "user-2@example.test")
	authService := newTestAuthService(sharedauth.NewTokenService(testJWTKey))
	return New(Config{JWTSecret: testJWTKey, Logger: slog.Default(), AuthService: authService, DeviceRepository: deviceRepo})
}

func newCloudDeviceRepository(db *sql.DB) DeviceRepository {
	return clouddevice.NewRepository(db, "sqlite")
}

func newCloudTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Open sqlite error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	statements := []string{
		`PRAGMA foreign_keys = ON`,
		`CREATE TABLE users (id TEXT PRIMARY KEY, email_normalized TEXT NOT NULL UNIQUE, display_name TEXT NOT NULL DEFAULT '', status TEXT NOT NULL, email_verified_at TEXT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL, last_login_at TEXT NULL)`,
		`CREATE TABLE user_identities (id TEXT PRIMARY KEY, user_id TEXT NOT NULL, provider TEXT NOT NULL, provider_subject TEXT NOT NULL, password_hash TEXT NULL, oauth_email TEXT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL, UNIQUE(provider, provider_subject), FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE)`,
		`CREATE TABLE devices (id TEXT PRIMARY KEY, name TEXT NOT NULL, public_key TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL, updated_at TEXT NOT NULL)`,
		`CREATE TABLE user_devices (user_id TEXT NOT NULL, device_id TEXT NOT NULL, role TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL, PRIMARY KEY (user_id, device_id), FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE, FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE)`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("exec schema statement error = %v\n%s", err, statement)
		}
	}
	return db
}

func insertCloudUser(t *testing.T, db *sql.DB, userId string, email string) {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := db.Exec(`INSERT INTO users (id,email_normalized,display_name,status,email_verified_at,created_at,updated_at) VALUES (?,?,?,?,?,?,?)`, userId, email, email, "enabled", now, now, now); err != nil {
		t.Fatalf("insert cloud user %s error = %v", userId, err)
	}
	if _, err := db.Exec(`INSERT INTO user_identities (id,user_id,provider,provider_subject,password_hash,created_at,updated_at) VALUES (?,?,?,?,?,?,?)`, userId+"-email", userId, "email", email, "", now, now); err != nil {
		t.Fatalf("insert cloud user identity %s error = %v", userId, err)
	}
}

func cloudUserToken(t *testing.T, authService AuthService, userId string, email string) string {
	t.Helper()
	token, err := sharedauth.NewTokenService(testJWTKey).Sign(sharedauth.Claims{Sub: userId, Email: email, Provider: "email"})
	if err != nil {
		t.Fatalf("Sign token error = %v", err)
	}
	if _, err := authService.VerifyToken(token); err != nil {
		t.Fatalf("VerifyToken() error = %v", err)
	}
	return token
}

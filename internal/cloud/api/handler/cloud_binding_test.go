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

	clouddevice "termbridge-go/internal/cloud/repository/user/device"
	sharedauth "termbridge-go/internal/shared/common/auth"
	"termbridge-go/internal/shared/dto/protocol/tunnel"
)

const registeredCloudOAuthRedirectURL = "http://localhost:9031/cloud/oauth/callback"

func TestCloudOAuthAuthorizeRejectsInvalidRegisteredClientRequest(t *testing.T) {
	tests := []struct {
		name  string
		query string
	}{
		{name: "missing client id", query: "redirect_uri=http://localhost:9031/cloud/oauth/callback&state=state-1"},
		{name: "wrong client id", query: "client_id=other-client&redirect_uri=http://localhost:9031/cloud/oauth/callback&state=state-1"},
		{name: "wrong redirect uri", query: "client_id=termbridge-local&redirect_uri=http://127.0.0.1:9031/cloud/oauth/callback&state=state-1"},
		{name: "missing state", query: "client_id=termbridge-local&redirect_uri=http://localhost:9031/cloud/oauth/callback"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := newCloudHandlerForTest(t)
			token := cloudUserToken(t, handler.authService, "user-1", "user-1@example.test")
			request := httptest.NewRequest(http.MethodGet, "/api/cloud-oauth/authorize?"+tt.query, nil)
			request.Header.Set("Authorization", "Bearer "+token)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			assertAPIError(t, response, http.StatusBadRequest, errorCodeBadRequest)
		})
	}
}

func TestCloudOAuthAuthorizeRejectsUnauthenticatedRequest(t *testing.T) {
	handler := newCloudHandlerForTest(t)
	request := httptest.NewRequest(http.MethodGet, "/api/cloud-oauth/authorize?client_id=termbridge-local&redirect_uri=http://localhost:9031/cloud/oauth/callback&state=state-1", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	assertAPIError(t, response, http.StatusUnauthorized, errorCodeUnauthorized)
}

func TestCloudOAuthAuthorizeExchangeAndCurrentDeviceReport(t *testing.T) {
	handler := newCloudHandlerForTest(t)
	token := cloudUserToken(t, handler.authService, "user-1", "user-1@example.test")

	authorizeRequest := httptest.NewRequest(http.MethodGet, "/api/cloud-oauth/authorize?client_id=termbridge-local&redirect_uri=http://localhost:9031/cloud/oauth/callback&state=state-1", nil)
	authorizeRequest.Header.Set("Authorization", "Bearer "+token)
	authorizeResponse := httptest.NewRecorder()
	handler.ServeHTTP(authorizeResponse, authorizeRequest)
	if authorizeResponse.Code != http.StatusOK {
		t.Fatalf("authorize status = %d; body=%s", authorizeResponse.Code, authorizeResponse.Body.String())
	}
	var authorizeBody struct {
		RedirectURL string `json:"redirect_url"`
	}
	if err := json.Unmarshal(authorizeResponse.Body.Bytes(), &authorizeBody); err != nil {
		t.Fatalf("decode authorize response: %v", err)
	}
	if !strings.HasPrefix(authorizeBody.RedirectURL, registeredCloudOAuthRedirectURL+"?") || !strings.Contains(authorizeBody.RedirectURL, "state=state-1") || !strings.Contains(authorizeBody.RedirectURL, "code=") {
		t.Fatalf("authorize redirect_url = %q", authorizeBody.RedirectURL)
	}
	callbackRequest := httptest.NewRequest(http.MethodGet, authorizeBody.RedirectURL, nil)
	code := callbackRequest.URL.Query().Get("code")

	exchangeResponse := httptest.NewRecorder()
	handler.ServeHTTP(exchangeResponse, httptest.NewRequest(http.MethodPost, "/api/cloud-oauth/exchange", strings.NewReader(`{"code":"`+code+`"}`)))
	if exchangeResponse.Code != http.StatusOK {
		t.Fatalf("exchange status = %d; body=%s", exchangeResponse.Code, exchangeResponse.Body.String())
	}
	var tokenBody struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(exchangeResponse.Body.Bytes(), &tokenBody); err != nil {
		t.Fatalf("decode exchange response: %v", err)
	}
	if tokenBody.AccessToken == "" {
		t.Fatal("exchange response access_token is empty")
	}

	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	localDevice := Device{ID: "dev-1", Name: "local-device", PublicKey: base64.StdEncoding.EncodeToString(publicKey)}
	reportRequest := httptest.NewRequest(http.MethodPost, "/api/devices/current", strings.NewReader(`{"id":"`+localDevice.ID+`","name":"`+localDevice.Name+`","public_key":"`+localDevice.PublicKey+`"}`))
	reportRequest.Header.Set("Authorization", "Bearer "+tokenBody.AccessToken)
	reportResponse := httptest.NewRecorder()
	handler.ServeHTTP(reportResponse, reportRequest)
	if reportResponse.Code != http.StatusOK {
		t.Fatalf("current device report status = %d; body=%s", reportResponse.Code, reportResponse.Body.String())
	}

	devicesRequest := httptest.NewRequest(http.MethodGet, "/api/devices", nil)
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
	if len(devices.Items) != 1 || devices.Items[0].Id != localDevice.ID || devices.Items[0].Online {
		t.Fatalf("devices = %#v", devices)
	}
	storedPublicKey, err := handler.config.DeviceRepository.PublicKey(context.Background(), localDevice.ID)
	if err != nil {
		t.Fatalf("PublicKey(%s) error = %v", localDevice.ID, err)
	}
	if storedPublicKey != localDevice.PublicKey {
		t.Fatalf("PublicKey(%s) = %q", localDevice.ID, storedPublicKey)
	}
	handler.config.AgentTunnelAudience = "test-audience"
	tunnelHeader, err := tunnel.SignedTunnelHeader(http.MethodGet, "/api/agent/tunnel", "test-audience", localDevice.ID, privateKey, time.Now(), "test-nonce")
	if err != nil {
		t.Fatalf("SignedTunnelHeader() error = %v", err)
	}
	tunnelRequest := httptest.NewRequest(http.MethodGet, "/api/agent/tunnel", nil)
	tunnelRequest.Header = tunnelHeader
	verifiedDeviceId, ok := handler.verifyAgentTunnelRequest(tunnelRequest)
	if !ok || verifiedDeviceId != localDevice.ID {
		t.Fatalf("verifyAgentTunnelRequest() = %q, %v; want %q, true", verifiedDeviceId, ok, localDevice.ID)
	}

	reuseResponse := httptest.NewRecorder()
	handler.ServeHTTP(reuseResponse, httptest.NewRequest(http.MethodPost, "/api/cloud-oauth/exchange", strings.NewReader(`{"code":"`+code+`"}`)))
	assertAPIError(t, reuseResponse, http.StatusUnauthorized, errorCodeUnauthorized)
}

func TestDevicesFiltersByCloudUserAndDeleteDisconnectsRoute(t *testing.T) {
	handler := newCloudHandlerForTest(t)
	repo := handler.config.DeviceRepository
	ctx := context.Background()
	if err := repo.UpsertDeviceBinding(ctx, "user-1", Device{ID: "dev-1", Name: "owned", PublicKey: base64.StdEncoding.EncodeToString([]byte("12345678901234567890123456789012"))}); err != nil {
		t.Fatalf("UpsertDeviceBinding(user-1) error = %v", err)
	}
	if err := repo.UpsertDeviceBinding(ctx, "user-2", Device{ID: "dev-2", Name: "other", PublicKey: base64.StdEncoding.EncodeToString([]byte("abcdefghijklmnopqrstuvwxyz123456"))}); err != nil {
		t.Fatalf("UpsertDeviceBinding(user-2) error = %v", err)
	}
	handler.registry.Register("dev-1", "owned", time.Now().UTC())
	handler.setRoute("dev-1", newAgentRoute("dev-1", nil))
	token := cloudUserToken(t, handler.authService, "user-1", "user-1@example.test")

	devicesRequest := httptest.NewRequest(http.MethodGet, "/api/devices", nil)
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

	deleteRequest := httptest.NewRequest(http.MethodDelete, "/api/devices/dev-1", nil)
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
	return New(Config{JWTSecret: testJWTKey, Logger: slog.Default(), AuthService: authService, DeviceRepository: deviceRepo, CloudOAuth: CloudOAuthConfig{ClientID: "termbridge-local", RedirectURL: registeredCloudOAuthRedirectURL}})
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
		`CREATE TABLE device_binding_codes (code_hash TEXT PRIMARY KEY, user_id TEXT NOT NULL, used_at TEXT NULL, expires_at TEXT NOT NULL, created_at TEXT NOT NULL, FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE)`,
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

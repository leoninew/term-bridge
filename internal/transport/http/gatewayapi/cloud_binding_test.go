package gatewayapi

import (
	"context"
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

	authapp "termbridge-go/internal/application/auth"
	"termbridge-go/internal/infrastructure/config"
	authrepo "termbridge-go/internal/infrastructure/repository/auth"
	devicerepo "termbridge-go/internal/infrastructure/repository/device"
	gatewayauth "termbridge-go/internal/transport/http/gatewayapi/auth"
)

func TestCloudConnectStartReturnsAuthorizeRedirectInLocalMode(t *testing.T) {
	authService := authapp.New(nil, gatewayauth.NewTokenService("test-secret"), config.AuthConfig{}, "local", nil, nil)
	gateway := New(Config{JWTSecret: "test-secret", Logger: slog.Default(), AuthService: authService, CloudBindingAttemptStore: authapp.NewCloudBindingAttemptStore(t.TempDir()), CloudGateURL: "https://cloud.example.test", CloudOAuth: CloudOAuthConfig{ClientID: "termbridge-local", RedirectURL: "http://127.0.0.1:9030/cloud/connect/callback", Scopes: []string{"openid", "email", "profile"}}, WebMode: "local"})

	request := httptest.NewRequest(http.MethodGet, "/cloud/connect/start", nil)
	response := httptest.NewRecorder()
	gateway.ServeHTTP(response, request)
	if response.Code != http.StatusFound {
		t.Fatalf("cloud connect start status = %d; body=%s", response.Code, response.Body.String())
	}
	location := response.Header().Get("Location")
	if !strings.HasPrefix(location, "https://cloud.example.test/oauth2/authorize?") || !strings.Contains(location, "client_id=termbridge-local") || !strings.Contains(location, "redirect_uri=http%3A%2F%2F127.0.0.1%3A9030%2Fcloud%2Fconnect%2Fcallback") || !strings.Contains(location, "response_type=code") || !strings.Contains(location, "scope=openid+email+profile") || !strings.Contains(location, "state=") {
		t.Fatalf("Location = %q", location)
	}
}

func TestCloudConnectAuthorizeRejectsNonLoopbackCallback(t *testing.T) {
	gateway := newCloudGatewayForTest(t)
	token := cloudUserToken(t, gateway.authService, "user-1", "user-1@example.test")
	request := httptest.NewRequest(http.MethodGet, "/api/cloud-connect/authorize?redirect_uri=https://evil.example.test/callback&state=state-1", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	gateway.ServeHTTP(response, request)
	assertAPIError(t, response, http.StatusBadRequest, errorCodeBadRequest)
}

func TestCloudConnectAuthorizeExchangeAndCurrentDeviceReport(t *testing.T) {
	gateway := newCloudGatewayForTest(t)
	token := cloudUserToken(t, gateway.authService, "user-1", "user-1@example.test")

	authorizeRequest := httptest.NewRequest(http.MethodGet, "/api/cloud-connect/authorize?redirect_uri=http://127.0.0.1:9030/cloud/connect/callback&state=state-1", nil)
	authorizeRequest.Header.Set("Authorization", "Bearer "+token)
	authorizeResponse := httptest.NewRecorder()
	gateway.ServeHTTP(authorizeResponse, authorizeRequest)
	if authorizeResponse.Code != http.StatusOK {
		t.Fatalf("authorize status = %d; body=%s", authorizeResponse.Code, authorizeResponse.Body.String())
	}
	var authorizeBody struct {
		RedirectURL string `json:"redirect_url"`
	}
	if err := json.Unmarshal(authorizeResponse.Body.Bytes(), &authorizeBody); err != nil {
		t.Fatalf("decode authorize response: %v", err)
	}
	if !strings.Contains(authorizeBody.RedirectURL, "state=state-1") || !strings.Contains(authorizeBody.RedirectURL, "code=") {
		t.Fatalf("authorize redirect_url = %q", authorizeBody.RedirectURL)
	}
	callbackRequest := httptest.NewRequest(http.MethodGet, authorizeBody.RedirectURL, nil)
	code := callbackRequest.URL.Query().Get("code")

	exchangeResponse := httptest.NewRecorder()
	gateway.ServeHTTP(exchangeResponse, httptest.NewRequest(http.MethodPost, "/api/cloud-connect/exchange", strings.NewReader(`{"code":"`+code+`"}`)))
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

	reportRequest := httptest.NewRequest(http.MethodPost, "/api/devices/current", strings.NewReader(`{"id":"dev-1","name":"local"}`))
	reportRequest.Header.Set("Authorization", "Bearer "+tokenBody.AccessToken)
	reportResponse := httptest.NewRecorder()
	gateway.ServeHTTP(reportResponse, reportRequest)
	if reportResponse.Code != http.StatusOK {
		t.Fatalf("current device report status = %d; body=%s", reportResponse.Code, reportResponse.Body.String())
	}

	devicesRequest := httptest.NewRequest(http.MethodGet, "/api/devices", nil)
	devicesRequest.Header.Set("Authorization", "Bearer "+token)
	devicesResponse := httptest.NewRecorder()
	gateway.ServeHTTP(devicesResponse, devicesRequest)
	if devicesResponse.Code != http.StatusOK {
		t.Fatalf("devices status = %d; body=%s", devicesResponse.Code, devicesResponse.Body.String())
	}
	var devices []DeviceSummary
	if err := json.Unmarshal(devicesResponse.Body.Bytes(), &devices); err != nil {
		t.Fatalf("decode devices: %v", err)
	}
	if len(devices) != 1 || devices[0].Id != "dev-1" || devices[0].Online {
		t.Fatalf("devices = %#v", devices)
	}

	reuseResponse := httptest.NewRecorder()
	gateway.ServeHTTP(reuseResponse, httptest.NewRequest(http.MethodPost, "/api/cloud-connect/exchange", strings.NewReader(`{"code":"`+code+`"}`)))
	assertAPIError(t, reuseResponse, http.StatusUnauthorized, errorCodeUnauthorized)
}

func TestDevicesFiltersByCloudUserAndDeleteDisconnectsRoute(t *testing.T) {
	gateway := newCloudGatewayForTest(t)
	repo := gateway.config.DeviceRepository
	ctx := context.Background()
	if err := repo.UpsertDeviceBinding(ctx, "user-1", devicerepo.Device{ID: "dev-1", Name: "owned", PublicKey: base64.StdEncoding.EncodeToString([]byte("12345678901234567890123456789012"))}); err != nil {
		t.Fatalf("UpsertDeviceBinding(user-1) error = %v", err)
	}
	if err := repo.UpsertDeviceBinding(ctx, "user-2", devicerepo.Device{ID: "dev-2", Name: "other", PublicKey: base64.StdEncoding.EncodeToString([]byte("abcdefghijklmnopqrstuvwxyz123456"))}); err != nil {
		t.Fatalf("UpsertDeviceBinding(user-2) error = %v", err)
	}
	gateway.registry.Register("dev-1", "owned", time.Now().UTC())
	gateway.setRoute("dev-1", newAgentRoute("dev-1", nil))
	token := cloudUserToken(t, gateway.authService, "user-1", "user-1@example.test")

	devicesRequest := httptest.NewRequest(http.MethodGet, "/api/devices", nil)
	devicesRequest.Header.Set("Authorization", "Bearer "+token)
	devicesResponse := httptest.NewRecorder()
	gateway.ServeHTTP(devicesResponse, devicesRequest)
	if devicesResponse.Code != http.StatusOK {
		t.Fatalf("devices status = %d; body=%s", devicesResponse.Code, devicesResponse.Body.String())
	}
	var devices []DeviceSummary
	if err := json.Unmarshal(devicesResponse.Body.Bytes(), &devices); err != nil {
		t.Fatalf("decode devices: %v", err)
	}
	if len(devices) != 1 || devices[0].Id != "dev-1" || !devices[0].Online {
		t.Fatalf("devices = %#v", devices)
	}

	deleteRequest := httptest.NewRequest(http.MethodDelete, "/api/devices/dev-1", nil)
	deleteRequest.Header.Set("Authorization", "Bearer "+token)
	deleteResponse := httptest.NewRecorder()
	gateway.ServeHTTP(deleteResponse, deleteRequest)
	if deleteResponse.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d; body=%s", deleteResponse.Code, deleteResponse.Body.String())
	}
	if route := gateway.routeFor("dev-1"); route != nil {
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

func newCloudGatewayForTest(t *testing.T) *Handler {
	t.Helper()
	db := newGatewayTestDB(t)
	authRepo := authrepo.New(db, "sqlite")
	deviceRepo := devicerepo.New(db, "sqlite")
	insertGatewayUser(t, db, "user-1", "user-1@example.test")
	insertGatewayUser(t, db, "user-2", "user-2@example.test")
	authService := authapp.New(authRepo, gatewayauth.NewTokenService("test-secret"), config.AuthConfig{}, "cloud", nil, nil)
	return New(Config{JWTSecret: "test-secret", Logger: slog.Default(), AuthService: authService, DeviceRepository: deviceRepo, WebMode: "cloud"})
}

func newGatewayTestDB(t *testing.T) *sql.DB {
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
		`CREATE TABLE devices (id TEXT PRIMARY KEY, name TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL)`,
		`CREATE TABLE user_devices (user_id TEXT NOT NULL, device_id TEXT NOT NULL, role TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL, PRIMARY KEY (user_id, device_id), FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE, FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE)`,
		`CREATE TABLE device_keys (id TEXT PRIMARY KEY, device_id TEXT NOT NULL, public_key TEXT NOT NULL, created_at TEXT NOT NULL, FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE)`,
		`CREATE INDEX idx_device_keys_device ON device_keys(device_id, created_at)`,
		`CREATE TABLE device_binding_codes (code_hash TEXT PRIMARY KEY, user_id TEXT NOT NULL, used_at TEXT NULL, expires_at TEXT NOT NULL, created_at TEXT NOT NULL, FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE)`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("exec schema statement error = %v\n%s", err, statement)
		}
	}
	return db
}

func insertGatewayUser(t *testing.T, db *sql.DB, userId string, email string) {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := db.Exec(`INSERT INTO users (id,email_normalized,display_name,status,email_verified_at,created_at,updated_at) VALUES (?,?,?,?,?,?,?)`, userId, email, email, "enabled", now, now, now); err != nil {
		t.Fatalf("insert gateway user %s error = %v", userId, err)
	}
	if _, err := db.Exec(`INSERT INTO user_identities (id,user_id,provider,provider_subject,password_hash,created_at,updated_at) VALUES (?,?,?,?,?,?,?)`, userId+"-email", userId, "email", email, "", now, now); err != nil {
		t.Fatalf("insert gateway user identity %s error = %v", userId, err)
	}
}

func cloudUserToken(t *testing.T, authService *authapp.Service, userId string, email string) string {
	t.Helper()
	token, err := gatewayauth.NewTokenService("test-secret").Sign(gatewayauth.Claims{Sub: userId, Email: email, Provider: "email"})
	if err != nil {
		t.Fatalf("Sign token error = %v", err)
	}
	if _, err := authService.VerifyToken(token); err != nil {
		t.Fatalf("VerifyToken() error = %v", err)
	}
	return token
}

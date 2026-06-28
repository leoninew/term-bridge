package gatewayapi

import (
	"bytes"
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

func TestCloudBindingStartRequiresLocalAccessAndReturnsAuthorizeURL(t *testing.T) {
	setupStore := authapp.NewSetupTokenStore(t.TempDir())
	setupToken, err := setupStore.Create(authapp.SetupTokenTTL)
	if err != nil {
		t.Fatalf("Create setup token error = %v", err)
	}
	authService := authapp.New(nil, gatewayauth.NewTokenService("test-secret"), config.AuthConfig{}, "local", nil, nil).WithLocalSetup(setupStore, "dev-1", "local")
	token, err := authService.CompleteLocalSetup(context.Background(), setupToken)
	if err != nil {
		t.Fatalf("CompleteLocalSetup() error = %v", err)
	}
	gateway := New(Config{JWTSecret: "test-secret", Logger: slog.Default(), AuthService: authService, CloudBindingAttemptStore: authapp.NewCloudBindingAttemptStore(t.TempDir()), CloudGateURL: "https://cloud.example.test", CloudCallbackBaseURL: "http://127.0.0.1:9030"})

	request := httptest.NewRequest(http.MethodPost, "/api/cloud-binding/start", nil)
	request.Header.Set("Authorization", "Bearer "+token.Token)
	response := httptest.NewRecorder()
	gateway.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("cloud binding start status = %d; body=%s", response.Code, response.Body.String())
	}
	var body struct {
		AuthorizeURL string `json:"authorize_url"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !strings.HasPrefix(body.AuthorizeURL, "https://cloud.example.test/device-bindings/authorize?") || !strings.Contains(body.AuthorizeURL, "callback=http%3A%2F%2F127.0.0.1%3A9030%2Fapi%2Fcloud-binding%2Fcallback") || !strings.Contains(body.AuthorizeURL, "state=") {
		t.Fatalf("authorize_url = %q", body.AuthorizeURL)
	}
}

func TestCloudBindingAuthorizeRejectsNonLoopbackCallback(t *testing.T) {
	gateway := newCloudGatewayForTest(t)
	token := cloudUserToken(t, gateway.authService, "user-1", "user-1@example.test")
	request := httptest.NewRequest(http.MethodGet, "/api/device-bindings/authorize?callback=https://evil.example.test/callback&state=state-1", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	gateway.ServeHTTP(response, request)
	assertAPIError(t, response, http.StatusBadRequest, errorCodeBadRequest)
}

func TestCloudBindingAuthorizeAndExchangeCreatesUserDevice(t *testing.T) {
	gateway := newCloudGatewayForTest(t)
	token := cloudUserToken(t, gateway.authService, "user-1", "user-1@example.test")

	authorizeRequest := httptest.NewRequest(http.MethodGet, "/api/device-bindings/authorize?callback=http://127.0.0.1:9030/api/cloud-binding/callback&state=state-1", nil)
	authorizeRequest.Header.Set("Authorization", "Bearer "+token)
	authorizeResponse := httptest.NewRecorder()
	gateway.ServeHTTP(authorizeResponse, authorizeRequest)
	if authorizeResponse.Code != http.StatusFound {
		t.Fatalf("authorize status = %d; body=%s", authorizeResponse.Code, authorizeResponse.Body.String())
	}
	location := authorizeResponse.Header().Get("Location")
	if location == "" || !strings.Contains(location, "state=state-1") || !strings.Contains(location, "code=") {
		t.Fatalf("authorize location = %q", location)
	}

	jsonAuthorizeRequest := httptest.NewRequest(http.MethodGet, "/api/device-bindings/authorize?callback=http://127.0.0.1:9030/api/cloud-binding/callback&state=state-2", nil)
	jsonAuthorizeRequest.Header.Set("Authorization", "Bearer "+token)
	jsonAuthorizeRequest.Header.Set("X-TermBridge-Authorize-Mode", "json")
	jsonAuthorizeResponse := httptest.NewRecorder()
	gateway.ServeHTTP(jsonAuthorizeResponse, jsonAuthorizeRequest)
	if jsonAuthorizeResponse.Code != http.StatusOK {
		t.Fatalf("json authorize status = %d; body=%s", jsonAuthorizeResponse.Code, jsonAuthorizeResponse.Body.String())
	}
	var jsonAuthorizeBody struct {
		RedirectURL string `json:"redirect_url"`
	}
	if err := json.Unmarshal(jsonAuthorizeResponse.Body.Bytes(), &jsonAuthorizeBody); err != nil {
		t.Fatalf("decode json authorize response: %v", err)
	}
	if !strings.Contains(jsonAuthorizeBody.RedirectURL, "state=state-2") || !strings.Contains(jsonAuthorizeBody.RedirectURL, "code=") {
		t.Fatalf("json authorize redirect_url = %q", jsonAuthorizeBody.RedirectURL)
	}
	callbackRequest := httptest.NewRequest(http.MethodGet, location, nil)
	code := callbackRequest.URL.Query().Get("code")

	exchangeResponse := httptest.NewRecorder()
	gateway.ServeHTTP(exchangeResponse, httptest.NewRequest(http.MethodPost, "/api/device-bindings/exchange", bytes.NewBufferString(`{"code":"`+code+`","device":{"id":"dev-1","name":"local","public_key":"`+base64.StdEncoding.EncodeToString([]byte("12345678901234567890123456789012"))+`"}}`)))
	if exchangeResponse.Code != http.StatusOK {
		t.Fatalf("exchange status = %d; body=%s", exchangeResponse.Code, exchangeResponse.Body.String())
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
	gateway.ServeHTTP(reuseResponse, httptest.NewRequest(http.MethodPost, "/api/device-bindings/exchange", bytes.NewBufferString(`{"code":"`+code+`","device":{"id":"dev-2","name":"local","public_key":"key"}}`)))
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
	authService := authapp.New(authRepo, gatewayauth.NewTokenService("test-secret"), config.AuthConfig{}, "remote", nil, nil)
	return New(Config{JWTSecret: "test-secret", Logger: slog.Default(), AuthService: authService, DeviceRepository: deviceRepo})
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

func insertGatewayUser(t *testing.T, db *sql.DB, userID string, email string) {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := db.Exec(`INSERT INTO users (id,email_normalized,display_name,status,email_verified_at,created_at,updated_at) VALUES (?,?,?,?,?,?,?)`, userID, email, email, "enabled", now, now, now); err != nil {
		t.Fatalf("insert gateway user %s error = %v", userID, err)
	}
}

func cloudUserToken(t *testing.T, authService *authapp.Service, userID string, email string) string {
	t.Helper()
	token, err := gatewayauth.NewTokenService("test-secret").Sign(gatewayauth.Claims{Sub: userID, Email: email, Provider: "email"})
	if err != nil {
		t.Fatalf("Sign token error = %v", err)
	}
	if _, err := authService.VerifyToken(token); err != nil {
		t.Fatalf("VerifyToken() error = %v", err)
	}
	return token
}

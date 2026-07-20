package api

import (
	"context"
	"crypto/ed25519"
	"database/sql"
	"encoding/base64"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	clouddevice "gitee.com/leoninew/TermBridge-go/internal/cloud/repository/user/device"
	cloud "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/cloud/v1"
	sharedauth "gitee.com/leoninew/TermBridge-go/internal/shared/common/auth"
	"gitee.com/leoninew/TermBridge-go/internal/shared/common/security"
	"gitee.com/leoninew/TermBridge-go/internal/shared/common/utils/codec"
	"gitee.com/leoninew/TermBridge-go/internal/shared/dto/protocol/tunnel"
)

func TestCurrentDeviceReportBindsDeviceToCloudUserAndTunnelUsesPublicKey(t *testing.T) {
	handler := newCloudHandlerForTest(t)
	token := cloudUserToken(t, handler.authService, "user-1", "user-1@example.test")
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	localDevice := testDeviceForPublicKey(t, publicKey, "local-device")

	reportRequest := httptest.NewRequest(http.MethodPost, "/api/devices/current", strings.NewReader(`{"id":"`+localDevice.Id+`","name":"`+localDevice.Name+`","public_key":"`+localDevice.PublicKey+`"}`))
	reportRequest.Header.Set("Authorization", "Bearer "+token)
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
	var devices cloud.ListDevicesResp
	if err := codec.UnmarshalProtoJSON(devicesResponse.Body.Bytes(), &devices); err != nil {
		t.Fatalf("decode devices: %v", err)
	}
	if len(devices.GetItems()) != 1 || devices.GetItems()[0].GetId() != localDevice.Id || devices.GetItems()[0].GetOnline() {
		t.Fatalf("devices count=%d first_id=%q first_online=%v", len(devices.GetItems()), devices.GetItems()[0].GetId(), devices.GetItems()[0].GetOnline())
	}
	storedPublicKey, err := handler.config.DeviceRepository.PublicKey(context.Background(), localDevice.Id)
	if err != nil {
		t.Fatalf("PublicKey(%s) error = %v", localDevice.Id, err)
	}
	if storedPublicKey != localDevice.PublicKey {
		t.Fatalf("PublicKey(%s) = %q", localDevice.Id, storedPublicKey)
	}
	handler.config.AgentTunnelAudience = "test-audience"
	tunnelHeader, err := tunnel.SignedTunnelHeader(http.MethodGet, "/api/agent/tunnel", "test-audience", localDevice.Id, privateKey, time.Now(), "test-nonce")
	if err != nil {
		t.Fatalf("SignedTunnelHeader() error = %v", err)
	}
	tunnelRequest := httptest.NewRequest(http.MethodGet, "/api/agent/tunnel", nil)
	tunnelRequest.Header = tunnelHeader
	verifiedDeviceId, ok := handler.verifyAgentTunnelRequest(tunnelRequest)
	if !ok || verifiedDeviceId != localDevice.Id {
		t.Fatalf("verifyAgentTunnelRequest() = %q, %v; want %q, true", verifiedDeviceId, ok, localDevice.Id)
	}
}

func testDeviceForPublicKey(t *testing.T, publicKey ed25519.PublicKey, name string) Device {
	t.Helper()
	id, err := security.DeviceIdForEd25519PublicKey(publicKey)
	if err != nil {
		t.Fatalf("DeviceIdForEd25519PublicKey() error = %v", err)
	}
	return Device{Id: id, Name: name, PublicKey: base64.StdEncoding.EncodeToString(publicKey)}
}

func TestCurrentDeviceReportAllowsSharedBindingAndRejectsPublicKeyConflict(t *testing.T) {
	handler := newCloudHandlerForTest(t)
	publicKey, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	device := testDeviceForPublicKey(t, publicKey, "shared-device")
	requestBody := `{"id":"` + device.Id + `","name":"` + device.Name + `","public_key":"` + device.PublicKey + `"}`
	for _, userId := range []string{"user-1", "user-2"} {
		request := httptest.NewRequest(http.MethodPost, "/api/devices/current", strings.NewReader(requestBody))
		request.Header.Set("Authorization", "Bearer "+cloudUserToken(t, handler.authService, userId, userId+"@example.test"))
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("report for %s status = %d; body=%s", userId, response.Code, response.Body.String())
		}
	}
	for _, userId := range []string{"user-1", "user-2"} {
		owns, err := handler.config.DeviceRepository.UserOwnsDevice(context.Background(), userId, device.Id)
		if err != nil || !owns {
			t.Fatalf("UserOwnsDevice(%s) = %v, %v", userId, owns, err)
		}
	}

	otherPublicKey, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey(other) error = %v", err)
	}
	conflictBody := `{"id":"` + device.Id + `","name":"changed","public_key":"` + base64.StdEncoding.EncodeToString(otherPublicKey) + `"}`
	conflictRequest := httptest.NewRequest(http.MethodPost, "/api/devices/current", strings.NewReader(conflictBody))
	conflictRequest.Header.Set("Authorization", "Bearer "+cloudUserToken(t, handler.authService, "user-1", "user-1@example.test"))
	conflictResponse := httptest.NewRecorder()
	handler.ServeHTTP(conflictResponse, conflictRequest)
	assertAPIError(t, conflictResponse, http.StatusConflict, errorCodeConflict)
	storedPublicKey, err := handler.config.DeviceRepository.PublicKey(context.Background(), device.Id)
	if err != nil || storedPublicKey != device.PublicKey {
		t.Fatalf("stored public key = %q, %v", storedPublicKey, err)
	}
}

func TestCurrentDeviceReportRejectsUnauthenticatedRequest(t *testing.T) {
	handler := newCloudHandlerForTest(t)
	request := httptest.NewRequest(http.MethodPost, "/api/devices/current", strings.NewReader(`{"id":"dev-1","name":"local-device","public_key":"public-key"}`))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	assertAPIError(t, response, http.StatusUnauthorized, errorCodeUnauthorized)
}

func TestDeleteSharedDeviceKeepsRouteUntilLastBinding(t *testing.T) {
	handler := newCloudHandlerForTest(t)
	privateKey := ed25519.NewKeyFromSeed([]byte("12345678901234567890123456789012"))
	device := testDeviceForPublicKey(t, privateKey.Public().(ed25519.PublicKey), "shared-device")
	for _, userId := range []string{"user-1", "user-2"} {
		if err := handler.config.DeviceRepository.UpsertDeviceBinding(context.Background(), userId, device); err != nil {
			t.Fatalf("UpsertDeviceBinding(%s) error = %v", userId, err)
		}
	}
	handler.registry.Register(device.Id, device.Name, time.Now().UTC())
	handler.setRoute(device.Id, newAgentRoute(device.Id, nil))

	for index, userId := range []string{"user-1", "user-2"} {
		request := httptest.NewRequest(http.MethodDelete, "/api/devices/"+device.Id, nil)
		request.Header.Set("Authorization", "Bearer "+cloudUserToken(t, handler.authService, userId, userId+"@example.test"))
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusNoContent {
			t.Fatalf("delete for %s status = %d; body=%s", userId, response.Code, response.Body.String())
		}
		if index == 0 && handler.routeFor(device.Id) == nil {
			t.Fatal("route removed while another user binding remains")
		}
	}
	if handler.routeFor(device.Id) != nil {
		t.Fatal("route remains after the last binding is deleted")
	}
}

func TestDevicesFiltersByCloudUserAndDeleteDisconnectsRoute(t *testing.T) {
	handler := newCloudHandlerForTest(t)
	repo := handler.config.DeviceRepository
	ctx := context.Background()
	privateKey := ed25519.NewKeyFromSeed([]byte("12345678901234567890123456789012"))
	owned := testDeviceForPublicKey(t, privateKey.Public().(ed25519.PublicKey), "owned")
	otherPrivateKey := ed25519.NewKeyFromSeed([]byte("abcdefghijklmnopqrstuvwxyz123456"))
	other := testDeviceForPublicKey(t, otherPrivateKey.Public().(ed25519.PublicKey), "other")
	if err := repo.UpsertDeviceBinding(ctx, "user-1", owned); err != nil {
		t.Fatalf("UpsertDeviceBinding(user-1) error = %v", err)
	}
	if err := repo.UpsertDeviceBinding(ctx, "user-2", other); err != nil {
		t.Fatalf("UpsertDeviceBinding(user-2) error = %v", err)
	}
	handler.registry.Register(owned.Id, "owned", time.Now().UTC())
	handler.setRoute(owned.Id, newAgentRoute(owned.Id, nil))
	token := cloudUserToken(t, handler.authService, "user-1", "user-1@example.test")

	devicesRequest := httptest.NewRequest(http.MethodGet, "/api/devices", nil)
	devicesRequest.Header.Set("Authorization", "Bearer "+token)
	devicesResponse := httptest.NewRecorder()
	handler.ServeHTTP(devicesResponse, devicesRequest)
	if devicesResponse.Code != http.StatusOK {
		t.Fatalf("devices status = %d; body=%s", devicesResponse.Code, devicesResponse.Body.String())
	}
	var devices cloud.ListDevicesResp
	if err := codec.UnmarshalProtoJSON(devicesResponse.Body.Bytes(), &devices); err != nil {
		t.Fatalf("decode devices: %v", err)
	}
	if len(devices.GetItems()) != 1 || devices.GetItems()[0].GetId() != owned.Id || !devices.GetItems()[0].GetOnline() {
		t.Fatalf("devices count=%d first_id=%q first_online=%v", len(devices.GetItems()), devices.GetItems()[0].GetId(), devices.GetItems()[0].GetOnline())
	}

	deleteRequest := httptest.NewRequest(http.MethodDelete, "/api/devices/"+owned.Id, nil)
	deleteRequest.Header.Set("Authorization", "Bearer "+token)
	deleteResponse := httptest.NewRecorder()
	handler.ServeHTTP(deleteResponse, deleteRequest)
	if deleteResponse.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d; body=%s", deleteResponse.Code, deleteResponse.Body.String())
	}
	if route := handler.routeFor(owned.Id); route != nil {
		t.Fatalf("route after delete = %#v, want nil", route)
	}
	owns, err := repo.UserOwnsDevice(ctx, "user-1", owned.Id)
	if err != nil {
		t.Fatalf("UserOwnsDevice(after delete) error = %v", err)
	}
	if owns {
		t.Fatalf("user-1 still owns %s after delete", owned.Id)
	}
}

func newCloudHandlerForTest(t *testing.T) *Handler {
	t.Helper()
	db := newCloudTestDB(t)
	deviceRepo := newCloudDeviceRepository(db)
	insertCloudUser(t, db, "user-1", "user-1@example.test")
	insertCloudUser(t, db, "user-2", "user-2@example.test")
	authService := newTestAuthService(sharedauth.NewTokenService(testJWTKey))
	return New(Config{Logger: slog.Default(), AuthService: authService, DeviceRepository: deviceRepo})
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

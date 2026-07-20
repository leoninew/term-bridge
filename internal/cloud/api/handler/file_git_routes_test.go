package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	clouddevice "gitee.com/leoninew/TermBridge-go/internal/cloud/repository/user/device"
	sharedauth "gitee.com/leoninew/TermBridge-go/internal/shared/common/auth"
)

type fileGitDeviceRepository struct {
	owns     bool
	lookups  int
	bindings int
}

func (r *fileGitDeviceRepository) UpsertDeviceBinding(context.Context, string, Device) error {
	r.bindings++
	return nil
}
func (r *fileGitDeviceRepository) UpsertUserDevice(context.Context, string, Device) error {
	return nil
}
func (r *fileGitDeviceRepository) UserOwnsDevice(context.Context, string, string) (bool, error) {
	r.lookups++
	return r.owns, nil
}
func (r *fileGitDeviceRepository) DeleteUserDevice(context.Context, string, string) (bool, error) {
	return true, nil
}
func (r *fileGitDeviceRepository) PublicKey(context.Context, string) (string, error) {
	return "", nil
}
func (r *fileGitDeviceRepository) ListDevicesForUser(context.Context, string) ([]clouddevice.Device, error) {
	return nil, nil
}

func TestFileGitRouteHidesUnownedDeviceBeforeTunnelLookup(t *testing.T) {
	repository := &fileGitDeviceRepository{}
	handler := newFileGitHandler(t, repository)
	handler.setRoute("dev-1", newAgentRoute("dev-1", nil))
	token := fileGitUserToken(t, handler, "user-1")

	request := httptest.NewRequest(http.MethodGet, "/api/devices/dev-1/workspaces/ws-1/fs/readDirectory?path=", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body=%s", response.Code, response.Body.String())
	}
	if repository.lookups != 1 {
		t.Fatalf("ownership lookups = %d, want 1", repository.lookups)
	}
	if route := handler.routeFor("dev-1"); route == nil {
		t.Fatal("the route was removed while handling an unowned request")
	}
}

func TestFileGitRouteReturnsDeviceOfflineForOwnedDevice(t *testing.T) {
	repository := &fileGitDeviceRepository{owns: true}
	handler := newFileGitHandler(t, repository)
	token := fileGitUserToken(t, handler, "user-1")

	request := httptest.NewRequest(http.MethodGet, "/api/devices/dev-1/workspaces/ws-1/scm/status", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503; body=%s", response.Code, response.Body.String())
	}
	if repository.lookups != 1 {
		t.Fatalf("ownership lookups = %d, want 1", repository.lookups)
	}
}

func newFileGitHandler(t *testing.T, repository DeviceRepository) *Handler {
	t.Helper()
	tokens := sharedauth.NewTokenService(testJWTKey)
	authService := newTestAuthService(tokens)
	turnstile, csrf := testAuthSecurityConfig()
	return New(Config{Logger: testCloudConfig().Logger, AuthService: authService, DeviceRepository: repository, Turnstile: turnstile, CSRF: csrf})
}

func fileGitUserToken(t *testing.T, handler *Handler, userId string) string {
	t.Helper()
	token, err := handler.authService.(*testAuthService).tokens.Sign(sharedauth.Claims{Sub: userId, Email: userId + "@example.test", Provider: "email"})
	if err != nil {
		t.Fatalf("sign user token: %v", err)
	}
	return token
}

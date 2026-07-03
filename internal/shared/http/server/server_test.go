package server

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestServerServesConfiguredHandler(t *testing.T) {
	server := New(Config{}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("ok"))
	}))

	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/any", nil))

	if response.Code != http.StatusCreated || response.Body.String() != "ok" {
		t.Fatalf("ServeHTTP() = %d %q, want handler response", response.Code, response.Body.String())
	}
}

func TestServerListenRejectsInvalidURL(t *testing.T) {
	server := New(Config{ServerUrl: "not-a-url"}, http.NotFoundHandler())
	listener, _, err := server.Listen()
	if err == nil {
		_ = listener.Close()
		t.Fatal("Listen() error = nil, want invalid server url error")
	}
}

func TestServerListenUsesTCPHost(t *testing.T) {
	server := New(Config{ServerUrl: "http://127.0.0.1:0"}, http.NotFoundHandler())
	listener, info, err := server.Listen()
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	defer listener.Close()
	if _, ok := listener.Addr().(*net.TCPAddr); !ok {
		t.Fatalf("listener addr = %T, want TCP", listener.Addr())
	}
	if info.Url != "http://127.0.0.1:0" {
		t.Fatalf("Info.Url = %q, want trimmed configured URL", info.Url)
	}
}

func TestServerServeStopsOnContextCancel(t *testing.T) {
	server := New(Config{ServerUrl: "http://127.0.0.1:0"}, http.NotFoundHandler())
	listener, _, err := server.Listen()
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx, listener) }()
	cancel()
	select {
	case err := <-done:
		if err != context.Canceled {
			t.Fatalf("Serve() error = %v, want context canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Serve() did not stop after context cancellation")
	}
}

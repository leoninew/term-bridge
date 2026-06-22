package terminalproto

import (
	"strings"
	"testing"
)

func TestDecodeClientAcceptsResize(t *testing.T) {
	message, err := DecodeClient([]byte(`{"type":"resize","cols":120,"rows":32}`))
	if err != nil {
		t.Fatalf("DecodeClient() error = %v", err)
	}
	if message.Type != TypeResize || message.Cols != 120 || message.Rows != 32 {
		t.Fatalf("message = %#v", message)
	}
}

func TestDecodeClientRejectsUnknownType(t *testing.T) {
	_, err := DecodeClient([]byte(`{"type":"bogus"}`))
	if err == nil {
		t.Fatal("DecodeClient() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("error = %v", err)
	}
}

func TestDecodeClientAcceptsLargeResize(t *testing.T) {
	message, err := DecodeClient([]byte(`{"type":"resize","cols":1200,"rows":600}`))
	if err != nil {
		t.Fatalf("DecodeClient() error = %v", err)
	}
	if message.Cols != 1200 || message.Rows != 600 {
		t.Fatalf("message = %#v", message)
	}
}

func TestDecodeClientRejectsResizeOutOfRange(t *testing.T) {
	_, err := DecodeClient([]byte(`{"type":"resize","cols":10001,"rows":32}`))
	if err == nil {
		t.Fatal("DecodeClient() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "cols out of range") {
		t.Fatalf("error = %v", err)
	}
}

func TestDecodeClientRejectsLargeControlMessage(t *testing.T) {
	_, err := DecodeClient([]byte(`{"type":"ping","nonce":"` + strings.Repeat("x", MaxJSONMessageBytes) + `"}`))
	if err == nil {
		t.Fatal("DecodeClient() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Fatalf("error = %v", err)
	}
}

func TestEncodeServerRejectsUnknownType(t *testing.T) {
	_, err := EncodeServer(ServerMessage{Type: "bogus"})
	if err == nil {
		t.Fatal("EncodeServer() error = nil, want error")
	}
}

package terminalproto

import (
	"strings"
	"testing"

	terminal "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/agent/v1"
)

func TestDecodeClientAcceptsResize(t *testing.T) {
	message, err := DecodeClient([]byte(`{"type":"resize","cols":120,"rows":32}`))
	if err != nil {
		t.Fatalf("DecodeClient() error = %v", err)
	}
	if message.Type != TypeResize || message.Cols != 120 || message.Rows != 32 {
		t.Fatalf("resize control message = %#v, want type resize with 120x32", message)
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
	message, err := DecodeClient([]byte(`{"type":"resize","cols":1000,"rows":600}`))
	if err != nil {
		t.Fatalf("DecodeClient() error = %v", err)
	}
	if message.Cols != 1000 || message.Rows != 600 {
		t.Fatalf("message = %#v", message)
	}
}

func TestDecodeClientRejectsResizeOutOfRange(t *testing.T) {
	_, err := DecodeClient([]byte(`{"type":"resize","cols":1001,"rows":32}`))
	if err == nil {
		t.Fatal("DecodeClient() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "cols out of range") {
		t.Fatalf("error = %v", err)
	}
}

func TestDecodeClientRejectsRowsOutOfRange(t *testing.T) {
	_, err := DecodeClient([]byte(`{"type":"resize","cols":120,"rows":1001}`))
	if err == nil {
		t.Fatal("DecodeClient() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "rows out of range") {
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
	_, err := EncodeServer(&terminal.ServerControlMessage{Type: "bogus"})
	if err == nil {
		t.Fatal("EncodeServer() error = nil, want error")
	}
}

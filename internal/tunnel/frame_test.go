package tunnel

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestEncodeDecodeFrame(t *testing.T) {
	frame, err := NewFrame(ControlStreamID, FrameHello, HelloPayload{DeviceID: "dev-1", DeviceName: "local", ProtocolVersion: ProtocolVersion})
	if err != nil {
		t.Fatalf("NewFrame() error = %v", err)
	}
	data, err := Encode(frame)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	decoded, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if decoded.StreamID != ControlStreamID || decoded.Type != FrameHello {
		t.Fatalf("decoded frame = %#v", decoded)
	}
	payload, err := DecodePayload[HelloPayload](decoded)
	if err != nil {
		t.Fatalf("DecodePayload() error = %v", err)
	}
	if payload.DeviceID != "dev-1" || payload.DeviceName != "local" || payload.ProtocolVersion != ProtocolVersion {
		t.Fatalf("payload = %#v", payload)
	}
}

func TestDecodeRejectsUnknownFrameType(t *testing.T) {
	_, err := Decode([]byte(`{"stream_id":"control","type":"bad"}`))
	if err == nil {
		t.Fatal("Decode() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "unknown tunnel frame type") {
		t.Fatalf("error = %v", err)
	}
}

func TestDecodeRejectsVersionMismatch(t *testing.T) {
	payload, err := json.Marshal(HelloPayload{DeviceID: "dev-1", DeviceName: "local", ProtocolVersion: ProtocolVersion + 1})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	frame := Frame{StreamID: ControlStreamID, Type: FrameHello, Payload: payload}
	data, err := json.Marshal(frame)
	if err != nil {
		t.Fatalf("Marshal(frame) error = %v", err)
	}
	_, err = Decode(data)
	if err == nil {
		t.Fatal("Decode() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "unsupported tunnel protocol version") {
		t.Fatalf("error = %v", err)
	}
}

func TestCloseAndErrorFrames(t *testing.T) {
	for _, frameType := range []FrameType{FrameClose, FrameError} {
		frame, err := NewFrame("term-1", frameType, ErrorPayload{Message: "closed"})
		if err != nil {
			t.Fatalf("NewFrame(%s) error = %v", frameType, err)
		}
		data, err := Encode(frame)
		if err != nil {
			t.Fatalf("Encode(%s) error = %v", frameType, err)
		}
		decoded, err := Decode(data)
		if err != nil {
			t.Fatalf("Decode(%s) error = %v", frameType, err)
		}
		if decoded.Type != frameType || decoded.StreamID != "term-1" {
			t.Fatalf("decoded = %#v", decoded)
		}
	}
}

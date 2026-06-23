package tunnel

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestEncodeDecodeFrame(t *testing.T) {
	frame, err := NewFrame(ControlStreamId, FrameHello, HelloPayload{DeviceId: "dev-1", DeviceName: "local", ProtocolVersion: ProtocolVersion})
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
	if decoded.StreamId != ControlStreamId || decoded.Type != FrameHello {
		t.Fatalf("decoded frame = %#v", decoded)
	}
	payload, err := DecodePayload[HelloPayload](decoded)
	if err != nil {
		t.Fatalf("DecodePayload() error = %v", err)
	}
	if payload.DeviceId != "dev-1" || payload.DeviceName != "local" || payload.ProtocolVersion != ProtocolVersion {
		t.Fatalf("payload = %#v", payload)
	}
}

func TestTerminalDataPayloadPreservesBytes(t *testing.T) {
	want := []byte{'h', 'i', 0xff, 0xfe, 0x00, 0x1b, '[', '2', 'J'}
	frame, err := NewFrame("term-1", FrameTerminalOutput, TerminalDataPayload{Data: want})
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
	payload, err := DecodePayload[TerminalDataPayload](decoded)
	if err != nil {
		t.Fatalf("DecodePayload() error = %v", err)
	}
	if !bytes.Equal(payload.Data, want) {
		t.Fatalf("payload.Data = %q, want %q", payload.Data, want)
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
	payload, err := json.Marshal(HelloPayload{DeviceId: "dev-1", DeviceName: "local", ProtocolVersion: ProtocolVersion + 1})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	frame := Frame{StreamId: ControlStreamId, Type: FrameHello, Payload: payload}
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
		if decoded.Type != frameType || decoded.StreamId != "term-1" {
			t.Fatalf("decoded = %#v", decoded)
		}
	}
}

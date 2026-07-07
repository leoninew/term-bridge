package codec

import (
	"strings"
	"testing"

	common "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/shared/v1"
)

func TestMarshalProtoJSONUsesProtoNames(t *testing.T) {
	data, err := MarshalProtoJSON(&common.ErrorResp{RequestId: "req-1"})
	if err != nil {
		t.Fatalf("MarshalProtoJSON() error = %v", err)
	}
	text := string(data)
	if !strings.Contains(text, `"request_id"`) {
		t.Fatalf("MarshalProtoJSON() = %s, want request_id field", text)
	}
	if strings.Contains(text, `"requestId"`) {
		t.Fatalf("MarshalProtoJSON() = %s, must not use requestId field", text)
	}
}

func TestUnmarshalProtoJSONRejectsUnknownFields(t *testing.T) {
	var resp common.ErrorResp
	err := UnmarshalProtoJSON([]byte(`{"request_id":"req-1","unknown_field":"x"}`), &resp)
	if err == nil {
		t.Fatal("UnmarshalProtoJSON() error = nil, want unknown field error")
	}
}

func TestDecodeJSONStruct(t *testing.T) {
	var target struct {
		Code string `json:"code"`
	}

	err := DecodeJSONStruct(strings.NewReader(`{"code":"oauth-code"}`), &target)
	if err != nil {
		t.Fatalf("DecodeJSONStruct() error = %v", err)
	}
	if target.Code != "oauth-code" {
		t.Fatalf("Code = %q, want oauth-code", target.Code)
	}
}

func TestDecodeJSONStructRejectsUnknownFields(t *testing.T) {
	var target struct {
		Code string `json:"code"`
	}

	err := DecodeJSONStruct(strings.NewReader(`{"code":"oauth-code","extra":"nope"}`), &target)
	if err == nil {
		t.Fatal("DecodeJSONStruct() error = nil, want unknown field error")
	}
}

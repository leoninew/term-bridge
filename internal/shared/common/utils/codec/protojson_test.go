package codec

import (
	"strings"
	"testing"

	commonv1 "termbridge/internal/gen/proto/termbridge/common/v1"
)

func TestMarshalProtoJSONUsesProtoNames(t *testing.T) {
	data, err := MarshalProtoJSON(&commonv1.ErrorResp{RequestId: "req-1"})
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
	var resp commonv1.ErrorResp
	err := UnmarshalProtoJSON([]byte(`{"request_id":"req-1","unknown_field":"x"}`), &resp)
	if err == nil {
		t.Fatal("UnmarshalProtoJSON() error = nil, want unknown field error")
	}
}

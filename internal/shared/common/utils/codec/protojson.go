package codec

import (
	"encoding/json"
	"fmt"
	"io"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

var MarshalOptions = protojson.MarshalOptions{
	UseProtoNames:   true,
	EmitUnpopulated: false,
}

var UnmarshalOptions = protojson.UnmarshalOptions{
	DiscardUnknown: false,
}

func MarshalProtoJSON(message proto.Message) ([]byte, error) {
	data, err := MarshalOptions.Marshal(message)
	if err != nil {
		return nil, fmt.Errorf("marshal proto json: %w", err)
	}
	return data, nil
}

func UnmarshalProtoJSON(data []byte, message proto.Message) error {
	if err := UnmarshalOptions.Unmarshal(data, message); err != nil {
		return fmt.Errorf("unmarshal proto json: %w", err)
	}
	return nil
}

func DecodeJSONStruct(reader io.Reader, target any) error {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode json struct: %w", err)
	}
	return nil
}

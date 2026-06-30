package security

import (
	"encoding/base64"
	"fmt"
)

func ParseBase64Key(value string, size int) ([]byte, error) {
	encodings := []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	}
	var decodeErr error
	for _, encoding := range encodings {
		key, err := encoding.DecodeString(value)
		if err != nil {
			decodeErr = err
			continue
		}
		if len(key) != size {
			return nil, fmt.Errorf("invalid key length: %d", len(key))
		}
		return key, nil
	}
	return nil, fmt.Errorf("invalid base64 key: %w", decodeErr)
}

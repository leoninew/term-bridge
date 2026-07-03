package idgen

import (
	"strings"
	"testing"

	"github.com/oklog/ulid/v2"
)

func TestUlidGeneratorReturnsParseableId(t *testing.T) {
	id, err := NewUlidGenerator().NewId()
	if err != nil {
		t.Fatalf("NewId() error = %v", err)
	}
	if _, err := ulid.ParseStrict(id); err != nil {
		t.Fatalf("ParseStrict(%q) error = %v", id, err)
	}
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}

func TestUlidGeneratorUsesUppercaseCanonicalEncoding(t *testing.T) {
	id, err := NewUlidGeneratorWithEntropy(zeroReader{}).NewId()
	if err != nil {
		t.Fatalf("NewId() error = %v", err)
	}
	if id != strings.ToUpper(id) {
		t.Fatalf("id = %q, want canonical uppercase", id)
	}
}

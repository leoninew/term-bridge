package identity

import (
	"crypto/rand"
	"io"
	"time"

	"github.com/oklog/ulid/v2"
)

type Generator interface {
	NewID() (string, error)
}

type ULIDGenerator struct {
	entropy io.Reader
}

func NewULIDGenerator() ULIDGenerator {
	return ULIDGenerator{entropy: rand.Reader}
}

func NewULIDGeneratorWithEntropy(entropy io.Reader) ULIDGenerator {
	if entropy == nil {
		entropy = rand.Reader
	}
	return ULIDGenerator{entropy: entropy}
}

func (g ULIDGenerator) NewID() (string, error) {
	id, err := ulid.New(ulid.Timestamp(time.Now().UTC()), g.entropy)
	if err != nil {
		return "", err
	}
	return id.String(), nil
}

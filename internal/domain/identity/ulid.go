package identity

import (
	"crypto/rand"
	"io"
	"time"

	"github.com/oklog/ulid/v2"
)

type Generator interface {
	NewId() (string, error)
}

type UlidGenerator struct {
	entropy io.Reader
}

func NewUlidGenerator() UlidGenerator {
	return UlidGenerator{entropy: rand.Reader}
}

func NewUlidGeneratorWithEntropy(entropy io.Reader) UlidGenerator {
	if entropy == nil {
		entropy = rand.Reader
	}
	return UlidGenerator{entropy: entropy}
}

func (g UlidGenerator) NewId() (string, error) {
	id, err := ulid.New(ulid.Timestamp(time.Now().UTC()), g.entropy)
	if err != nil {
		return "", err
	}
	return id.String(), nil
}

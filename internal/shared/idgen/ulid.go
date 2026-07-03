// Package idgen provides a single, dependency-free entry point for generating
// unique identifiers across the project. The only public API is [New].
package idgen

import (
	"crypto/rand"
	"io"
	"time"

	"github.com/oklog/ulid/v2"
)

// New returns a new ULID string using a shared monotonic entropy source.
// It is the project-wide entry point for generating unique identifiers.
func New() (string, error) {
	return idSource.NewId()
}

var idSource Generator = UlidGenerator{entropy: rand.Reader}

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

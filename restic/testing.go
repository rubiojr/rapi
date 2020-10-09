package restic

import (
	"fmt"
	"io"
	"math/rand"
)

// fakeFile returns a reader which yields deterministic pseudo-random data.
func fakeFile(seed, size int64) io.Reader {
	return io.LimitReader(rand.New(rand.NewSource(seed)), size)
}

const (
	maxFileSize = 20000
	maxSeed     = 32
	maxNodes    = 15
)

// TestParseID parses s as a ID and panics if that fails.
func TestParseID(s string) ID {
	id, err := ParseID(s)
	if err != nil {
		panic(fmt.Sprintf("unable to parse string %q as ID: %v", s, err))
	}

	return id
}

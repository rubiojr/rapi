package restic

import (
	"encoding/hex"

	"github.com/minio/sha256-simd"
)

const shortStr = 4

// idSize contains the size of an ID, in bytes.
const idSize = sha256.Size

// ID references content within a repository.
type ID [idSize]byte

// Hash returns the ID for data.
func Hash(data []byte) ID {
	return sha256.Sum256(data)
}

// Str returns the shortened string version of id.
func (id *ID) Str() string {
	if id == nil {
		return "[nil]"
	}

	if id.IsNull() {
		return "[null]"
	}

	return hex.EncodeToString(id[:shortStr])
}

// IsNull returns true iff id only consists of null bytes.
func (id ID) IsNull() bool {
	var nullID ID

	return id == nullID
}

func (id ID) String() string {
	return hex.EncodeToString(id[:])
}

// Equal compares an ID to another other.
func (id ID) Equal(other ID) bool {
	return id == other
}

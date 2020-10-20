package restic

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"

	"github.com/minio/sha256-simd"
	"github.com/pkg/errors"
)

const shortStr = 4

// idSize contains the size of an ID, in bytes.
const idSize = sha256.Size

// ID references content within a repository.
type ID [idSize]byte

// ParseID converts the given string to an ID.
func ParseID(s string) (ID, error) {
	b, err := hex.DecodeString(s)

	if err != nil {
		return ID{}, errors.Wrap(err, "hex.DecodeString")
	}

	if len(b) != idSize {
		return ID{}, errors.New("invalid length for hash")
	}

	id := ID{}
	copy(id[:], b)

	return id, nil
}

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

// MarshalJSON returns the JSON encoding of id.
func (id ID) MarshalJSON() ([]byte, error) {
	return json.Marshal(id.String())
}

// UnmarshalJSON parses the JSON-encoded data and stores the result in id.
func (id *ID) UnmarshalJSON(b []byte) error {
	// check string length
	if len(b) < 2 {
		return fmt.Errorf("invalid ID: %q", b)
	}

	if len(b)%2 != 0 {
		return fmt.Errorf("invalid ID length: %q", b)
	}

	// check string delimiters
	if b[0] != '"' && b[0] != '\'' {
		return fmt.Errorf("invalid start of string: %q", b[0])
	}

	last := len(b) - 1
	if b[0] != b[last] {
		return fmt.Errorf("starting string delimiter (%q) does not match end (%q)", b[0], b[last])
	}

	// strip JSON string delimiters
	b = b[1:last]

	if len(b) != 2*len(id) {
		return fmt.Errorf("invalid length for ID")
	}

	_, err := hex.Decode(id[:], b)
	if err != nil {
		return errors.Wrap(err, "hex.Decode")
	}

	return nil
}

func (id *ID) DirectoryPrefix() string {
	return id.String()[:2]
}

// NewRandomID returns a randomly generated ID. When reading from rand fails,
// the function panics.
func NewRandomID() ID {
	id := ID{}
	_, err := io.ReadFull(rand.Reader, id[:])
	if err != nil {
		panic(err)
	}
	return id
}

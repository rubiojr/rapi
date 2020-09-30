package blob

import (
	"errors"
	"fmt"
	"io"

	"github.com/rubiojr/rapi/crypto"
	"github.com/rubiojr/rapi/restic"
)

// Blob is one part of a file or a tree.
type Blob struct {
	Type   BlobType
	Length uint
	ID     restic.ID
	Offset uint
	PackID restic.ID
}

func (b Blob) String() string {
	return fmt.Sprintf("<Blob (%v) %v, offset %v, length %v>",
		b.Type, b.ID.Str(), b.Offset, b.Length)
}

// PackedBlob is a blob stored within a file.
type PackedBlob struct {
	Blob
	PackID restic.ID
}

// BlobHandle identifies a blob of a given type.
type BlobHandle struct {
	ID   restic.ID
	Type BlobType
}

func (h BlobHandle) String() string {
	return fmt.Sprintf("<%s/%s>", h.Type, h.ID.Str())
}

// BlobType specifies what a blob stored in a pack is.
type BlobType uint8

// These are the blob types that can be stored in a pack.
const (
	InvalidBlob BlobType = iota
	DataBlob
	TreeBlob
	NumBlobTypes // Number of types. Must be last in this enumeration.
)

func (t BlobType) String() string {
	switch t {
	case DataBlob:
		return "data"
	case TreeBlob:
		return "tree"
	case InvalidBlob:
		return "invalid"
	}

	return fmt.Sprintf("<BlobType %d>", t)
}

// MarshalJSON encodes the BlobType into JSON.
func (t BlobType) MarshalJSON() ([]byte, error) {
	switch t {
	case DataBlob:
		return []byte(`"data"`), nil
	case TreeBlob:
		return []byte(`"tree"`), nil
	}

	return nil, errors.New("unknown blob type")
}

// UnmarshalJSON decodes the BlobType from JSON.
func (t *BlobType) UnmarshalJSON(buf []byte) error {
	switch string(buf) {
	case `"data"`:
		*t = DataBlob
	case `"tree"`:
		*t = TreeBlob
	default:
		return errors.New("unknown blob type")
	}

	return nil
}

// BlobHandles is an ordered list of BlobHandles that implements sort.Interface.
type BlobHandles []BlobHandle

func (h BlobHandles) Len() int {
	return len(h)
}

func (h BlobHandles) Less(i, j int) bool {
	for k, b := range h[i].ID {
		if b == h[j].ID[k] {
			continue
		}

		if b < h[j].ID[k] {
			return true
		}

		return false
	}

	return h[i].Type < h[j].Type
}

func (h BlobHandles) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

func (h BlobHandles) String() string {
	elements := make([]string, 0, len(h))
	for _, e := range h {
		elements = append(elements, e.String())
	}
	return fmt.Sprintf("%v", elements)
}

// DecryptAndCheck decrypts the blob contents, optionally checking if the content
// is valid.
func (blob *Blob) DecryptAndCheck(reader io.ReaderAt, key *crypto.Key, check bool) ([]byte, error) {
	// load blob from pack

	buf := make([]byte, blob.Length)

	n, err := reader.ReadAt(buf, int64(blob.Offset))
	if err != nil {
		return nil, err
	}

	if uint(n) != blob.Length {
		return nil, fmt.Errorf("error loading blob %v: wrong length returned, want %d, got %d",
			blob.ID.Str(), blob.Length, uint(n))
	}

	// decrypt
	nonce, ciphertext := buf[:key.NonceSize()], buf[key.NonceSize():]
	plaintext, err := key.Open(ciphertext[:0], nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypting blob %v failed: %v", blob.ID, err)
	}

	if check && !restic.Hash(plaintext).Equal(blob.ID) {
		return nil, fmt.Errorf("blob %v returned invalid hash", blob.ID)
	}

	return plaintext, nil
}

// DecryptAndCheck decrypts the blob contents.
//
// Does not check content validity.
func (blob *Blob) Decrypt(reader io.ReaderAt, key *crypto.Key) ([]byte, error) {
	return blob.DecryptAndCheck(reader, key, false)
}

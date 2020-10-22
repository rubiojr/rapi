package restic

import (
	"fmt"
	"io"
	"os"

	"github.com/rubiojr/rapi/crypto"
)

func (id *ID) DirectoryPrefix() string {
	return id.String()[:2]
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

	if check && !Hash(plaintext).Equal(blob.ID) {
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

// DecryptAndCheck decrypts the blob contents.
//
// Does not check content validity.
func (blob *Blob) DecryptFromPack(path string, key *crypto.Key) ([]byte, error) {
	pack, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer pack.Close()

	return blob.DecryptAndCheck(pack, key, false)
}

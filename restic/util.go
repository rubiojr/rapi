package restic

import "github.com/rubiojr/rapi/crypto"

// CiphertextLength returns the encrypted length of a blob with plaintextSize
// bytes.
func CiphertextLength(plaintextSize int) int {
	return plaintextSize + crypto.Extension
}

// PlaintextLength returns the plaintext length of a blob with ciphertextSize
// bytes.
func PlaintextLength(ciphertextSize int) int {
	return ciphertextSize - crypto.Extension
}

// NewBlobBuffer returns a buffer that is large enough to hold a blob of size
// plaintext bytes, including the crypto overhead.
func NewBlobBuffer(size int) []byte {
	return make([]byte, size, size+crypto.Extension)
}

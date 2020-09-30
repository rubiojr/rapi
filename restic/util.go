package restic

import "github.com/rubiojr/rapi/crypto"

// CiphertextLength returns the encrypted length of a blob with plaintextSize
// bytes.
func CiphertextLength(plaintextSize int) int {
	return plaintextSize + crypto.Extension
}

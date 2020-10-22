package crypto

import (
	"fmt"
	"io"
	"io/ioutil"
)

func (k *Key) Decrypt(reader io.Reader) ([]byte, error) {
	buf, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	// decrypt
	nonce, ciphertext := buf[:k.NonceSize()], buf[k.NonceSize():]
	plaintext, err := k.Open(ciphertext[:0], nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decryption failed: %v", err)
	}

	return plaintext, nil
}

// Encrypt encrypts plaintext data and returns the ciphertext as a byte array
func (k *Key) Encrypt(data []byte) []byte {
	nonce := NewRandomNonce()

	ciphertext := make([]byte, 0, ciphertextLength(len(data)))
	ciphertext = append(ciphertext, nonce...)

	// encrypt blob
	ciphertext = k.Seal(ciphertext, nonce, data, nil)

	return ciphertext
}

func ciphertextLength(plaintextSize int) int {
	return plaintextSize + Extension
}

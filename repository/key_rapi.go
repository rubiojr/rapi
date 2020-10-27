package repository

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"os"

	"github.com/rubiojr/rapi/crypto"
	"github.com/rubiojr/rapi/internal/errors"
)

// rubiojr: added
func OpenKeyFromFile(file, password string) (*Key, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	k, err := Open(f, password)
	return k, err
}

// rubiojr: added
// Open tries do decrypt the key specified by name with the given password.
func Open(reader io.Reader, password string) (*Key, error) {
	k, err := loadKey(reader)
	if err != nil {
		return nil, err
	}

	// check KDF
	if k.KDF != "scrypt" {
		return nil, errors.New("only supported KDF is scrypt()")
	}

	// derive user key
	params := crypto.Params{
		N: k.N,
		R: k.R,
		P: k.P,
	}
	k.user, err = crypto.KDF(params, k.Salt, password)
	if err != nil {
		return nil, errors.Wrap(err, "crypto.KDF")
	}

	// decrypt master keys
	nonce, ciphertext := k.Data[:k.user.NonceSize()], k.Data[k.user.NonceSize():]
	buf, err := k.user.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	// restore json
	k.master = &crypto.Key{}
	err = json.Unmarshal(buf, k.master)
	if err != nil {
		return nil, errors.Wrap(err, "Unmarshal")
	}
	if !k.Valid() {
		return nil, errors.New("Invalid key for repository")
	}

	return k, nil
}

func loadKey(reader io.Reader) (k *Key, err error) {
	data, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	k = &Key{}
	err = json.Unmarshal(data, k)
	if err != nil {
		return nil, errors.Wrap(err, "Unmarshal")
	}

	return k, nil
}

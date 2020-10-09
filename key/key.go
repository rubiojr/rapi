package key

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"time"

	"github.com/pkg/errors"
	"github.com/rubiojr/rapi/crypto"
)

type Key struct {
	Created  time.Time `json:"created"`
	Username string    `json:"username"`
	Hostname string    `json:"hostname"`

	KDF  string `json:"kdf"`
	N    int    `json:"N"`
	R    int    `json:"r"`
	P    int    `json:"p"`
	Salt []byte `json:"salt"`
	Data []byte `json:"data"`

	User   *crypto.Key
	Master *crypto.Key
}

func OpenFromFile(file, password string) (*Key, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	k, err := Open(f, "test")
	return k, err
}

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
	k.User, err = crypto.KDF(params, k.Salt, password)
	if err != nil {
		return nil, errors.Wrap(err, "crypto.KDF")
	}

	// decrypt master keys
	nonce, ciphertext := k.Data[:k.User.NonceSize()], k.Data[k.User.NonceSize():]
	buf, err := k.User.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	// restore json
	k.Master = &crypto.Key{}
	err = json.Unmarshal(buf, k.Master)
	if err != nil {
		return nil, errors.Wrap(err, "Unmarshal")
	}
	if !k.Valid() {
		return nil, errors.New("Invalid key for repository")
	}

	return k, nil
}

func (k *Key) Valid() bool {
	return k.User.Valid() && k.Master.Valid()
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

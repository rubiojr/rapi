package util

import (
	"os"

	"github.com/rubiojr/rapi"
	"github.com/rubiojr/rapi/crypto"
)

var RepoPath = os.Getenv("RESTIC_REPOSITORY")
var RepoPass = os.Getenv("RESTIC_PASSWORD")

const MP3SHA256 = "01d4bac715e7cc70193fdf70db3c5022d0dd5f33dacd6d4a07a2747258416338"

// Find the first key listed available in restic's repository and open it so
// we can use it to encrypt/decrypt files
func FindAndOpenKey() *crypto.Key {
	repo, err := rapi.OpenRepository(rapi.DefaultOptions)
	CheckErr(err)
	return repo.Key()
}

func CheckErr(err error) {
	if err != nil {
		panic(err)
	}
}

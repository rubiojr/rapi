package util

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/rubiojr/rapi/key"
)

var RepoPath = os.Getenv("RESTIC_REPOSITORY")
var RepoPass = os.Getenv("RESTIC_PASSWORD")

const MP3SHA256 = "01d4bac715e7cc70193fdf70db3c5022d0dd5f33dacd6d4a07a2747258416338"

// Find the first key listed available in restic's repository and open it so
// we can use it to encrypt/decrypt files
func FindAndOpenKey() *key.Key {
	keyPath := findFirstRepositoryKey(RepoPath)
	k, err := key.OpenFromFile(keyPath, RepoPass)
	CheckErr(err)
	return k
}

func findFirstRepositoryKey(repoPath string) string {
	fi, err := ioutil.ReadDir(filepath.Join(repoPath, "keys"))
	CheckErr(err)

	for _, file := range fi {
		return filepath.Join(repoPath, "keys", file.Name())
	}
	return ""
}

func CheckErr(err error) {
	if err != nil {
		panic(err)
	}
}

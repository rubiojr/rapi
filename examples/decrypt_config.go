package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/rubiojr/rapi/key"
)

func main() {
	// get restic repository path and password from the environment
	repoPath := os.Getenv("RESTIC_REPOSITORY")
	repoPass := os.Getenv("RESTIC_PASSWORD")

	// our sample repo in /tmp/restic will only have one key, let's
	// find it and use it
	keyPath := findFirstKey(repoPath)
	k, err := key.OpenFromFile(keyPath, repoPass)
	checkErr(err)

	// open /tmp/restic/config file
	h, err := os.Open(filepath.Join(repoPath, "config"))
	checkErr(err)

	// decrypt the repo configuration, print it to stdout
	text, err := k.Master.Decrypt(h)
	checkErr(err)
	fmt.Println(string(text))
}

// Find first encryption key in the repository
func findFirstKey(repoPath string) string {
	fi, err := ioutil.ReadDir(filepath.Join(repoPath, "keys"))
	checkErr(err)

	for _, file := range fi {
		return filepath.Join(repoPath, "keys", file.Name())
	}
	return ""
}

func checkErr(err error) {
	if err != nil {
		panic(err)
	}
}

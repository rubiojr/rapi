package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/rubiojr/rapi/examples/util"
	"github.com/rubiojr/rapi/key"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Printf("Usage: encrypt_file <read_from> <write_to>\n")
		os.Exit(1)
	}
	in, out := os.Args[1], os.Args[2]

	// our sample repo in /tmp/restic will only have one key, let's
	// find it and use it
	keyPath := findFirstKey(util.RepoPath)
	k, err := key.OpenFromFile(keyPath, util.RepoPass)
	util.CheckErr(err)

	//
	h, err := os.Open(in)
	defer h.Close()
	util.CheckErr(err)

	// Read the file contents
	plain, err := ioutil.ReadAll(h)
	util.CheckErr(err)

	// Encrypt the file using restic's repository master key
	ciphertext := k.Master.Encrypt(plain)

	// Write the resulting ciphertext to the target file
	outf, err := os.Create(out)
	util.CheckErr(err)
	defer outf.Close()
	outf.Write(ciphertext)
}

// Find first encryption key in the repository
func findFirstKey(repoPath string) string {
	fi, err := ioutil.ReadDir(filepath.Join(repoPath, "keys"))
	util.CheckErr(err)

	for _, file := range fi {
		return filepath.Join(repoPath, "keys", file.Name())
	}
	return ""
}

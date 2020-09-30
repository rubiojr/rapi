package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/rubiojr/rapi/key"
	"github.com/rubiojr/rapi/pack"
)

func main() {
	repoPath := os.Getenv("RESTIC_REPOSITORY")
	repoPass := os.Getenv("RESTIC_PASSWORD")

	keyPath := findFirstKey(repoPath)
	k, err := key.OpenFromFile(keyPath, repoPass)
	checkErr(err)

	id := "8f46d6a57410e8a32675b1aa30b90118e0dc7d08afcc654426b5e00f3d817c02"
	idPre := id[:2]

	h, err := os.Open(filepath.Join(repoPath, "data", idPre, id))
	checkErr(err)

	s, err := h.Stat()
	checkErr(err)

	blobs, err := pack.List(k.Master, h, s.Size())
	checkErr(err)

	for _, blob := range blobs {
		// Describe blob and print content
		fmt.Println("Type: " + blob.Type.String())
		fmt.Println("ID: " + string(blob.ID.String()))
		buf := make([]byte, blob.Length)
		h.Read(buf)
		v, _ := blob.Decrypt(h, k.Master)
		fmt.Println("Content:\n" + string(v))
	}
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

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rubiojr/rapi/examples/util"
)

func main() {

	k := util.FindAndOpenKey()

	// open repository config file
	h, err := os.Open(filepath.Join(util.RepoPath, "config"))
	util.CheckErr(err)

	// decrypt the repo configuration, print it to stdout
	text, err := k.Decrypt(h)
	util.CheckErr(err)
	fmt.Println(string(text))
}

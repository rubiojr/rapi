package main

import (
	"fmt"
	"os"

	"github.com/rubiojr/rapi/examples/util"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Printf("Usage: encrypt_file <read_from>\n")
		os.Exit(1)
	}
	in := os.Args[1]

	k := util.FindAndOpenKey()

	h, err := os.Open(in)
	util.CheckErr(err)

	// decrypt the file using restic's repository master key, print it to stdout
	text, err := k.Master.Decrypt(h)
	util.CheckErr(err)
	fmt.Println(string(text))
}

package main

import (
	"context"
	"fmt"

	"github.com/rubiojr/rapi"
	"github.com/rubiojr/rapi/examples/util"
	"github.com/rubiojr/rapi/restic"
)

func main() {
	repo, err := rapi.OpenRepository(rapi.DefaultOptions)
	util.CheckErr(err)
	repo.LoadIndex(context.Background())
	index := repo.Index()

	dataBlob, _ := restic.ParseID("49bec000d8b727d3c50e8687e71b0a8deb65a84933f6c4dbbe07513ed39919cc")
	found := index.Has(dataBlob, restic.DataBlob)
	fmt.Printf("Is blob 49bec0 available in the index? %t\n", found)
	for _, blob := range index.Lookup(dataBlob, restic.DataBlob) {
		fmt.Printf("Blob %s found in pack file %s\n", blob.ID, blob.PackID)
	}

	fmt.Printf("Data blobs stored: %d\n", index.Count(restic.DataBlob))
	fmt.Printf("Tree blobs stored: %d\n", index.Count(restic.TreeBlob))
}

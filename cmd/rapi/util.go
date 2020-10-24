package main

import (
	"github.com/minio/sha256-simd"
	"github.com/rubiojr/rapi/restic"
)

// fileID is a 256-bit hash that distinguishes unique files.
type fileID [32]byte

// makeFileIDByContents returns a hash of the blob IDs of the
// node's Content in sequence.
func makeFileIDByContents(node *restic.Node) fileID {
	var bb []byte
	for _, c := range node.Content {
		bb = append(bb, []byte(c[:])...)
	}
	return sha256.Sum256(bb)
}

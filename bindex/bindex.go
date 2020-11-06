package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"github.com/rubiojr/rapi/repository"
	"github.com/rubiojr/rapi/restic"
)

// fileID is a 256-bit hash that distinguishes unique files.
type FileID struct {
	bytes [32]byte
}

type FileIndexer interface {
	Index(*restic.Node, chan *IndexResult)
	Repository() *repository.Repository
	WasIndexed(*FileID) (bool, error)
}

type IndexResult struct {
	Error error
	Node  *restic.Node
}

func NewFileID(bytes [32]byte) *FileID {
	return &FileID{bytes: bytes}
}

func (id *FileID) String() string {
	return fmt.Sprintf("%x", id.bytes)
}

func NodeFileID(node *restic.Node) *FileID {
	var bb []byte
	for _, c := range node.Content {
		bb = append(bb, []byte(c[:])...)
	}
	return NewFileID(sha256.Sum256(bb))
}

func MarshalBlobIDs(ids restic.IDs) string {
	j, err := json.Marshal(ids)
	if err != nil {
		panic(err)
	}
	return string(j)
}

package main

import (
	"context"
	"errors"

	"github.com/blugelabs/bluge"
	"github.com/rubiojr/rapi/repository"
	"github.com/rubiojr/rapi/restic"
)

type FileIndex struct {
	repository *repository.Repository
}

func (i *FileIndex) WasIndexed(id *FileID) (bool, error) {
	var err error

	query := bluge.NewWildcardQuery(id.String()).SetField("_id")
	request := bluge.NewAllMatches(query)

	documentMatchIterator, err := blugeReader().Search(context.Background(), request)
	if err != nil {
		panic(err)
	}

	match, err := documentMatchIterator.Next()
	if err == nil && match != nil {
		return true, nil
	}

	return false, nil
}

func NewFileIndexer(repo *repository.Repository) *FileIndex {
	f := &FileIndex{repository: repo}
	return f
}

func (i *FileIndex) Repository() *repository.Repository {
	return i.repository
}

func (i *FileIndex) Index(node *restic.Node, results chan *IndexResult) {
	if node == nil {
		results <- &IndexResult{Error: errors.New("nil node found"), Node: node}
		return
	}

	fileID := NodeFileID(node)
	lastScanned = node.Name
	if ok, _ := i.WasIndexed(fileID); ok {
		results <- &IndexResult{Error: errors.New("already indexed"), Node: node}
		return
	}

	nodeJSON, err := node.MarshalJSON()
	if err != nil {
		results <- &IndexResult{Error: err, Node: node}
		return
	}
	doc := bluge.NewDocument(fileID.String()).
		AddField(bluge.NewTextField("filename", string(node.Name)).StoreValue().HighlightMatches()).
		AddField(bluge.NewTextField("metadata", string(nodeJSON)).StoreValue().HighlightMatches()).
		AddField(bluge.NewTextField("repository_location", i.Repository().Backend().Location()).StoreValue().HighlightMatches()).
		AddField(bluge.NewTextField("repository_id", i.Repository().Config().ID).StoreValue().HighlightMatches())

	err = blugeWriter().Update(doc.ID(), doc)
	results <- &IndexResult{Error: err, Node: node}
}

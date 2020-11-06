package main

import (
	"github.com/blugelabs/bluge"
)

var bReader *bluge.Reader
var bWriter *bluge.Writer
var indexPath = "bluge.idx"
var blugeConf = bluge.DefaultConfig(indexPath)
var indexNeedsInit = true

func blugeReader() *bluge.Reader {
	r, err := blugeWriter().Reader()
	if err != nil {
		panic(err)
	}
	return r
}

func blugeWriter() *bluge.Writer {
	var err error
	if bWriter == nil {
		bWriter, err = bluge.OpenWriter(blugeConf)
		if err != nil {
			panic(err)
		}
		indexNeedsInit = false

	}

	return bWriter
}

.PHONY: all clean test restic

all: rapi

rapi: clean
	go build -o rapi ./cmd/rapi

clean:
	rm -f rapi

test: rapi
	./script/test

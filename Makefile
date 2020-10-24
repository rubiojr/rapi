.PHONY: all clean test restic

all: rapi

rapi:
	go build -o rapi ./cmd/rapi

clean:
	rm -f rapi

test:
	./script/test

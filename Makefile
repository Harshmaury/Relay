.PHONY: build test verify clean

build:
	go build -o relay ./cmd/relay/

test:
	go test ./...

verify:
	go vet ./...
	go build ./...

clean:
	rm -f relay

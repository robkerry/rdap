BINARY := rdap

.PHONY: test build clean

test:
	go test ./...

build:
	go build -trimpath -ldflags="-s -w" -o $(BINARY) ./cmd/rdap

clean:
	rm -f $(BINARY) rdap.exe

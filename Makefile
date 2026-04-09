.PHONY: run build test clean

run: build
	./kas $(ARGS)

build:
	go build -o kas ./cmd/kas

test:
	go test ./... -v

clean:
	rm -f kas kasmos

.PHONY: run build test test-fast test-full test-race bench-tests clean hooks

run: build
	./kas $(ARGS)

build:
	go build -o kas ./cmd/kas

test: test-fast

test-fast:
	go test ./...

test-full:
	go test -count=1 ./...

test-race:
	go test -race ./app ./session/tmux

bench-tests:
	./scripts/bench_tests.sh

hooks:
	bash scripts/git-hooks/install.sh

clean:
	rm -f kas kasmos

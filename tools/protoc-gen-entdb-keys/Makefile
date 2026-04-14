# protoc-gen-entdb-keys — protoc plugin that emits typed UniqueKey
# sidecar files for the EntDB SDK.
#
# POSIX-compliant — works on both macOS (BSD make) and Linux (GNU make).

BIN := bin/protoc-gen-entdb-keys

.PHONY: all build test install clean

all: build

build:
	@mkdir -p bin
	go build -o $(BIN) ./...

test:
	go test ./...

install:
	go install ./...

clean:
	rm -rf bin

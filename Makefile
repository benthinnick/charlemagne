# -race needs cgo and a 64-bit C toolchain; override with TESTFLAGS= on hosts
# that do not have one.
TESTFLAGS ?= -race

.PHONY: build test vet tidy

build:
	go build -trimpath -ldflags "-s -w" -o bin/charlemagne ./cmd/charlemagne

test:
	go test ./... $(TESTFLAGS) -count=1

vet:
	go vet ./...

tidy:
	go mod tidy

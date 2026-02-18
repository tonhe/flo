BINARY  := flo
GOFLAGS := -trimpath
VERSION := 0.1.0
BUILD   := $(shell date +%m%d%Y.%H%M)
LDFLAGS := -s -w -X github.com/tonhe/flo/internal/version.Version=$(VERSION) -X github.com/tonhe/flo/internal/version.Build=$(BUILD)

.PHONY: build clean install

build:
	go build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o $(BINARY) .

clean:
	rm -f $(BINARY)

install: build
	mkdir -p ~/bin
	cp $(BINARY) ~/bin/$(BINARY)

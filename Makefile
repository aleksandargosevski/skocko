BINARY    := skocko
VERSION   := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS   := -s -w -X skocko/cmd.version=$(VERSION)
GOPATH    := $(shell go env GOPATH)
CONFIG    := $(HOME)/.config/skocko/skocko.yaml


.PHONY: build install clean test dev


build:
	go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) .


install: build config
	@mkdir -p $(GOPATH)/bin
	rm -f $(GOPATH)/bin/$(BINARY)
	cp bin/$(BINARY) $(GOPATH)/bin/$(BINARY)
	@echo "Installed $(BINARY) to $(GOPATH)/bin/$(BINARY)"


config:
	@if [ ! -f $(CONFIG) ]; then \
		mkdir -p $(dir $(CONFIG)); \
		echo "Created $(CONFIG)"; \
	fi


clean:
	rm -rf bin/


test:
	go test ./...


dev: build
	./bin/$(BINARY)

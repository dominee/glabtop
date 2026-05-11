# glabtop — development targets
# Requires Go 1.22+

GO      ?= go
BINARY  ?= glabtop
CMD     := ./cmd/glabtop

.PHONY: help all build install test vet fmt fmt-check clean tidy run

help:
	@echo "Targets:"
	@echo "  make build     - build ./$(BINARY)"
	@echo "  make install   - install with $(GO) install"
	@echo "  make test      - $(GO) test ./..."
	@echo "  make vet       - $(GO) vet ./..."
	@echo "  make fmt       - gofmt -w ."
	@echo "  make fmt-check - fail if gofmt would change files (CI)"
	@echo "  make tidy      - $(GO) mod tidy"
	@echo "  make clean     - remove ./$(BINARY)"
	@echo "  make all       - fmt-check, vet, test, build (same as CI)"
	@echo "  make run       - build and run ./$(BINARY) (pass ARGS='...')"

all: fmt-check vet test build

build:
	$(GO) build -o $(BINARY) $(CMD)

install:
	$(GO) install $(CMD)

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

fmt:
	gofmt -w .

fmt-check:
	@test -z "$$(gofmt -l .)" || (echo "gofmt needed on:" && gofmt -l . && exit 1)

clean:
	rm -f $(BINARY)

tidy:
	$(GO) mod tidy

run: build
	./$(BINARY) $(ARGS)

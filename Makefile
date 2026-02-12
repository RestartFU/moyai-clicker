GO ?= go
APP ?= clicker
CMD ?= ./cmd/clicker
PREFIX ?= /usr/local
BINDIR ?= $(PREFIX)/bin
DESTDIR ?=
INSTALL ?= install

.PHONY: help build run test fmt vet clean rebuild install

help:
	@echo "Targets:"
	@echo "  build    Build $(APP)"
	@echo "  install  Install $(APP) to $(DESTDIR)$(BINDIR)"
	@echo "  run      Run the app from source"
	@echo "  test     Run tests"
	@echo "  fmt      Format Go code"
	@echo "  vet      Run go vet"
	@echo "  clean    Remove build artifacts"
	@echo "  rebuild  Clean then build"

build:
	$(GO) build -o $(APP) $(CMD)

install: build
	mkdir -p $(DESTDIR)$(BINDIR)
	$(INSTALL) -m 0755 $(APP) $(DESTDIR)$(BINDIR)/$(APP)

run:
	$(GO) run $(CMD)

test:
	$(GO) test ./...

fmt:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...

clean:
	rm -f $(APP)

rebuild: clean build

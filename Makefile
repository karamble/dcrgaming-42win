.PHONY: check test desktop dev play preview

GO ?= go
BIN ?= /tmp/dcr4inarow-bin

check:
	$(GO) test -race ./...
	$(GO) vet ./...
	mkdir -p $(BIN)
	$(GO) build -buildvcs=false -tags desktop -o $(BIN)/dcr4inarow ./cmd/dcr4inarow
	$(GO) build -buildvcs=false -tags desktop,dev -o $(BIN)/dcr4inarow-dev ./cmd/dcr4inarow

test:
	$(GO) test ./...

desktop:
	mkdir -p $(BIN)
	$(GO) build -buildvcs=false -tags desktop -o $(BIN)/dcr4inarow ./cmd/dcr4inarow

dev:
	mkdir -p $(BIN)
	$(GO) build -buildvcs=false -tags desktop,dev -o $(BIN)/dcr4inarow-dev ./cmd/dcr4inarow

# The local fixture board: no log, no stake, no bridge.
play: dev
	$(BIN)/dcr4inarow-dev -dev-board

# Table screens to PNG, with no display.
preview:
	mkdir -p artifacts
	$(GO) run ./cmd/dcr4inarow-preview -stage opening -hover 3 -output artifacts/dcr4inarow-opening.png
	$(GO) run ./cmd/dcr4inarow-preview -stage midgame -hover 4 -output artifacts/dcr4inarow-table.png
	$(GO) run ./cmd/dcr4inarow-preview -stage won -hover -1 -output artifacts/dcr4inarow-won.png
	$(GO) run ./cmd/dcr4inarow-preview -stage lobby -output artifacts/dcr4inarow-lobby.png
	$(GO) run ./cmd/dcr4inarow-preview -stage settings -output artifacts/dcr4inarow-settings.png
	for stage in invitation admission roster draw stake ready closed stale; do \
		$(GO) run ./cmd/dcr4inarow-preview -stage table-$$stage \
			-output artifacts/dcr4inarow-seating-$$stage.png; \
	done

# Two independent wallets, bridges and games on simnet. Builds the whole stack
# from the sibling source trees and needs docker. Nothing here touches mainnet.
.PHONY: acceptance
acceptance:
	bash simnet/run-acceptance.sh

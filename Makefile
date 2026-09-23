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
	mkdir -p artifacts $(BIN)
	$(GO) build -o $(BIN)/dcr4inarow-preview ./cmd/dcr4inarow-preview
	$(BIN)/dcr4inarow-preview -stage midgame -hover 4 -output artifacts/dcr4inarow-table.png
	for stage in opening won lobby lobby-connected settings cover cover-loading cover-error help pending disconnected round-win lost void spending spent blocked receipt; do \
		$(BIN)/dcr4inarow-preview -stage $$stage -output artifacts/dcr4inarow-$$stage.png; \
	done
	for stage in invitation admission roster draw stake ready closed stale; do \
		$(BIN)/dcr4inarow-preview -stage table-$$stage \
			-output artifacts/dcr4inarow-seating-$$stage.png; \
	done
	$(BIN)/dcr4inarow-preview -stage table-details -output artifacts/dcr4inarow-terms.png
	for size in 1280x800 1920x1080; do \
		for stage in cover lobby settings table-stake midgame won receipt; do \
			$(BIN)/dcr4inarow-preview -stage $$stage -width $${size%x*} -height $${size#*x} -output artifacts/dcr4inarow-$$stage-$$size.png; \
		done; \
	done
	mkdir -p artifacts/four2win
	for stage in cover cover-loading lobby settings table-stake midgame won receipt; do \
		$(BIN)/dcr4inarow-preview -stage $$stage -width 1280 -height 800 \
			-output artifacts/four2win/$$stage.png; \
	done

# Requires a desktop display. Uses a temporary profile and no bridge or wallet.
.PHONY: ui-check
ui-check:
	$(GO) test -tags desktop,dev ./cmd/dcr4inarow -count=1 -v

# Two independent wallets, bridges and games on simnet. Builds the whole stack
# from the sibling source trees and needs docker. Nothing here touches mainnet.
.PHONY: acceptance
acceptance:
	bash simnet/run-acceptance.sh

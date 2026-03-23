.PHONY: fmt fmt-check clippy build test integration-test clean

CARGO := cargo
CARGO_FLAGS := --manifest-path packages/contracts/Cargo.toml
WASM_TARGET := wasm32-unknown-unknown

fmt:
	$(CARGO) fmt --all $(CARGO_FLAGS)

fmt-check:
	$(CARGO) fmt --all $(CARGO_FLAGS) -- --check

clippy:
	$(CARGO) clippy $(CARGO_FLAGS) --all-targets --all-features -- -D warnings

build:
	$(CARGO) build $(CARGO_FLAGS) --target $(WASM_TARGET) --release

test:
	$(CARGO) test $(CARGO_FLAGS) --all

integration-test:
	$(CARGO) test $(CARGO_FLAGS) --test '*'

clean:
	$(CARGO) clean $(CARGO_FLAGS)

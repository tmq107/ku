BINARY := kli

.PHONY: build install run test vet tidy clean

build:
	go build -o $(BINARY) .

# install picks a destination directory by precedence (override with PREFIX=...):
#   1) ~/.local/bin, if it is on $PATH
#   2) /usr/local/bin, if it exists
#   3) the last directory on $PATH
install: build
	@dest="$(PREFIX)"; \
	if [ -z "$$dest" ]; then \
		case ":$$PATH:" in *":$$HOME/.local/bin:"*) dest="$$HOME/.local/bin" ;; esac; \
	fi; \
	if [ -z "$$dest" ] && [ -d /usr/local/bin ]; then dest="/usr/local/bin"; fi; \
	if [ -z "$$dest" ]; then dest="$$(printf '%s' "$$PATH" | tr ':' '\n' | grep -v '^$$' | tail -n1)"; fi; \
	if [ -z "$$dest" ]; then echo "install: could not determine a bin directory"; exit 1; fi; \
	mkdir -p "$$dest" && install -m 0755 $(BINARY) "$$dest/$(BINARY)" && \
		echo "installed $(BINARY) -> $$dest/$(BINARY)"

run:
	go run .

test:
	go test ./...

vet:
	go vet ./...

tidy:
	go mod tidy

clean:
	rm -f $(BINARY)

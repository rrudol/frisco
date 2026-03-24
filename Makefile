BIN_DIR := bin
CMDS := frisco

.PHONY: build run clean test vet

build:
	mkdir -p $(BIN_DIR)
	for cmd in $(CMDS); do go build -o $(BIN_DIR)/$$cmd ./cmd/$$cmd; done

run:
	go run ./cmd/frisco

clean:
	rm -rf $(BIN_DIR)

test:
	go test ./...

vet:
	go vet ./...

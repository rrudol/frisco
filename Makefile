BIN_DIR := bin

.PHONY: build run clean

build:
	mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/frisco ./cmd/frisco

run:
	go run ./cmd/frisco

clean:
	rm -rf $(BIN_DIR)

.PHONY: build clean install test deps

KIRO_DIR := $(HOME)/.kiro

deps:
	go mod download

build: deps
	go build -o budgie ./cmd/server

clean:
	rm -f budgie
	rm -rf bin/

install: build
	mkdir -p ~/.local/bin
	cp budgie ~/.local/bin/
	chmod +x ~/.local/bin/budgie
	cp hook-notify.sh ~/.local/bin/
	chmod +x ~/.local/bin/hook-notify.sh
	mkdir -p $(KIRO_DIR)/agents
	mkdir -p $(KIRO_DIR)/sub-agents/prompts
	mkdir -p $(KIRO_DIR)/prompts
	@for file in agents/config/*.json; do \
		sed -e "s|{{KIRO_DIR}}|$(KIRO_DIR)|g" \
		    -e "s|{{BUDGIE_BINARY}}|$(HOME)/.local/bin/budgie|g" \
		    -e "s|{{HOOK_NOTIFY}}|$(HOME)/.local/bin/hook-notify.sh|g" \
		    "$$file" > "$(KIRO_DIR)/agents/$$(basename $$file)"; \
	done
	cp agents/prompts/*.md $(KIRO_DIR)/sub-agents/prompts/
	-cp prompts/*.md $(KIRO_DIR)/prompts/

# Cross-compile for multiple platforms
build-all:
	mkdir -p bin
	GOOS=darwin GOARCH=arm64 go build -o bin/budgie-darwin-arm64 ./cmd/server
	GOOS=darwin GOARCH=amd64 go build -o bin/budgie-darwin-amd64 ./cmd/server
	GOOS=linux GOARCH=amd64 go build -o bin/budgie-linux-amd64 ./cmd/server
	GOOS=linux GOARCH=arm64 go build -o bin/budgie-linux-arm64 ./cmd/server
	GOOS=windows GOARCH=amd64 go build -o bin/budgie-windows-amd64.exe ./cmd/server

test:
	go test ./...

.PHONY: build install clean release test teammate

APP=daily-work
OUT=dist

build:
	go build -o bin/$(APP) ./cmd/daily-work

install:
	go install ./cmd/daily-work

# One-command setup for teammates (build + PATH + Ollama + model)
teammate:
	./install.sh

test:
	go test ./...

clean:
	rm -rf bin dist

release:
	mkdir -p $(OUT)
	GOOS=darwin GOARCH=arm64 go build -o $(OUT)/$(APP)-darwin-arm64 ./cmd/daily-work
	GOOS=darwin GOARCH=amd64 go build -o $(OUT)/$(APP)-darwin-amd64 ./cmd/daily-work
	GOOS=linux GOARCH=amd64 go build -o $(OUT)/$(APP)-linux-amd64 ./cmd/daily-work
	GOOS=linux GOARCH=arm64 go build -o $(OUT)/$(APP)-linux-arm64 ./cmd/daily-work
	GOOS=windows GOARCH=amd64 go build -o $(OUT)/$(APP)-windows-amd64.exe ./cmd/daily-work

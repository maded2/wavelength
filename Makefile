.PHONY: build run test clean

build:
	go build -o wl cmd/server/main.go

run: build
	./wl -config configs/config.json

test:
	go test ./...

clean:
	rm -f wl

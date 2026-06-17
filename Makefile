.PHONY: build run test clean

build:
	go build -o drxpkg main.go

run: build
	./drxpkg

test:
	go test ./...

clean:
	rm -f drxpkg

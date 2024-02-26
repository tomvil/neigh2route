VERSION=1.0.0
PACKAGES_DIR=compiled_packages

all: test build

test:
	go fmt ./
	go vet -v ./

clean:
	rm -f ${PACKAGES_DIR}/*

run:
	go run .

build:
	GOARCH=amd64 GOOS=linux go build -o ${PACKAGES_DIR}/neigh2route-${VERSION}-linux

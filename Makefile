NAME=kd
BINARY ?= ${NAME}
ROOT_DIR=${PWD}
HARDWARE=$(shell uname -m)
GIT_VERSION=$(shell git describe --always --tags --dirty)
GIT_SHA=$(shell git rev-parse HEAD)
GOVERSION=1.15
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%I:%M:%S%p')
VERSION ?= ${TRAVIS_TAG:-git+${TRAVIS_COMMIT:-local+${GIT_VERSION}}}
DEPS=$(shell go list -f '{{range .TestImports}}{{.}} {{end}}' ./...)
PACKAGES=$(shell go list ./...)
GOFILES_NOVENDOR=$(shell find . -type f -name '*.go' -not -path "./vendor/*")
VERSION_PKG=main
LFLAGS ?= -X ${VERSION_PKG}.Version=${GIT_VERSION}
VETARGS ?= -asmdecl -atomic -bool -buildtags -copylocks -methods -nilfunc -printf -rangeloops -unsafeptr

.PHONY: test changelog build release lint cover vet

default: build

testall: test src

golang:
	@echo "--> Go Version"
	@go version

build:
	@echo "--> Compiling the project"
	mkdir -p bin
	go build -ldflags "${LFLAGS}" -o bin/${NAME} ./...

release: clean
	@echo "--> Compiling all the static binaries"
	mkdir -p bin
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-w ${LFLAGS}" -o ./bin/kd_linux_amd64 ./...
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags "-w ${LFLAGS}" -o ./bin/kd_darwin_amd64 ./...
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "-w ${LFLAGS}" -o ./bin/kd_windows_amd64.exe ./...
	cd ./bin && sha256sum * > checksum.txt && cd -

docker-build:
	@echo "--> Building the docker image"
	docker build . -t "${NAME}:ci"

scan:
	@echo "--> Scanning the docker image via Anchore"
	mkdir -p images && rm -f images/${NAME}+ci.tar
	docker save ${NAME}:ci -o images/${NAME}+ci.tar
	chmod -R 0755 images/
	curl -s https://ci-tools.anchore.io/inline_scan-v0.4.1 | bash -s -- -v ./images -t 500

clean:
	rm -rf ./bin 2>/dev/null
	go mod tidy

vet:
	@echo "--> Running go vet $(VETARGS) ."
	@go tool vet 2>/dev/null ; if [ $$? -eq 3 ]; then \
		go get golang.org/x/tools/cmd/vet; \
	fi
	@go vet $(VETARGS) $(PACKAGES)

lint:
	@echo "--> Running golint"
	@which golint 2>/dev/null ; if [ $$? -eq 1 ]; then \
		go get -u golang.org/x/lint/golint; \
	fi
	@golint -set_exit_status .

gofmt:
	@echo "--> Running gofmt check"
	@gofmt -s -l $(GOFILES_NOVENDOR) | grep -q \.go ; if [ $$? -eq 0 ]; then \
      echo "we have unformatted files - run 'make applygofmt' to apply"; \
			gofmt -s -d -l ${GOFILES_NOVENDOR}; \
      exit 1; \
    fi

applygofmt:
	@echo "--> Running gofmt apply"
	@gofmt -s -l -w $(GOFILES_NOVENDOR)

bench:
	@echo "--> Running go bench"
	@go test -v -bench=.

coverage:
	@echo "--> Running go coverage"
	@go test -coverprofile cover.out
	@go tool cover -html=cover.out -o cover.html

cover:
	@echo "--> Running go cover"
	@go test -cover $(PACKAGES)

test:
	@echo "--> Running the tests"
	@go test -v ${PACKAGES}
	@$(MAKE) cover

src:
	@echo "--> Running the src checks"
	@$(MAKE) vet
	@$(MAKE) lint
	@$(MAKE) gofmt

changelog: release
	git log $(shell git tag | tail -n1)..HEAD --no-merges --format=%B > changelog

APP?=supervisord
USERSPACE?=serboox
RELEASE?=0.0.1
PROJECT?=github.com/${USERSPACE}/${APP}
BUILD_TIME?=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
GOOS?=linux
REPO_INFO=$(shell git config --get remote.origin.url)

ifndef COMMIT
	COMMIT := git-$(shell git rev-parse --short HEAD)
endif

# Default target executed when no arguments are given to make.
default_target: run

run: golangci-lint run-app
	@echo "+ $@"

run-app: clean build
	@echo "+ $@"
	./${APP}

run-test-http-server:
	@echo "+ $@"
	cd http-server \
		&& GO111MODULE=on golangci-lint run -c ../.golangci.yml ./... \
		&& go run main.go

golangci-lint: goimports
	@echo "+ $@"
	GO111MODULE=on golangci-lint run ./...

goimports:
	@echo "+ $@"
	goimports -d -w main.go ./src

clean:
	@echo "+ $@"
	rm -f ${APP}

kill:
	@echo "+ $@"
	lsof -i :${PORT} | grep ${APP} | awk '{print $$2}' | xargs kill -9

build: clean
	@echo "+ $@"
	GOFLAGS=-mod=vendor CGO_ENABLED=1 go build -v -installsuffix cgo \
		-ldflags "-s -w -X ${PROJECT}/src/version.Release=${RELEASE} -X ${PROJECT}/src/version.Commit=${COMMIT} -X ${PROJECT}/src/version.Repository=${REPO_INFO} -X ${PROJECT}/src/version.BuildTime=${BUILD_TIME}"





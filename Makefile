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

clean:
	@echo "+ $@"
	rm -f ${APP}

run-test-http-server:
	@echo "+ $@"
	cd http-server \
		&& GO111MODULE=on golangci-lint run -c ../.golangci.yml ./... \
		&& go run main.go

golangci-lint:
	@echo "+ $@"
	GO111MODULE=on golangci-lint run ./...



GIT_COMMIT=$(shell git rev-list -1 HEAD)
GIT_BRANCH=$(shell git branch --show-current)
LDFLAGS=-X main.GitCommit=${GIT_COMMIT} -X main.GitBranch=${GIT_BRANCH}

build:
	go build -ldflags "${LDFLAGS}" ./...

install:
	go install -ldflags "${LDFLAGS}" ./...

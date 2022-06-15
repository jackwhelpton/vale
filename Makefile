LAST_TAG=$(shell git describe --abbrev=0 --tags)
CURR_SHA=$(shell git rev-parse --verify HEAD)

LDFLAGS=-ldflags "-s -w -X main.version=$(LAST_TAG)"

.PHONY: data test lint install rules setup bench compare release

all: build

# make release tag=v0.4.3
release:
	git tag $(tag)
	git push origin $(tag)

# make build os=darwin
# make build os=windows
# make build os=linux
build:
	GOOS=$(os) GOARCH=amd64 go build ${LDFLAGS} -o bin/$(exe) ./cmd/vale

arm:
	GOOS=darwin GOARCH=arm64 go build ${LDFLAGS} -o bin/$(exe) ./cmd/vale

closed:
	GOOS=$(os) GOARCH=amd64 go build -tags closed ${LDFLAGS} -o bin/$(exe) ./cmd/vale

bench:
	go test -bench=. -benchmem ./core ./lint ./check

compare:
	cd lint && \
	benchmany -n 5 -o new.txt ${CURR_SHA} && \
	benchmany -n 5 -o old.txt ${LAST_TAG} && \
	benchcmp old.txt new.txt && \
	benchstat old.txt new.txt

setup:
	cd testdata && bundle install && cd -

rules:
	go-bindata -ignore=\\.DS_Store -pkg="rule" -o rule/rule.go rule/**/*.yml

data:
	go-bindata -ignore=\\.DS_Store -pkg="spell" -o pkg/spell/data.go pkg/spell/data/*.{dic,aff}

test:
	go test ./internal/core ./internal/lint ./internal/check ./internal/nlp ./pkg/glob
	cd testdata && cucumber --format progress && cd -


CWD := $(shell pwd)

all:
	GOPATH=$(GOPATH):$(CWD)/vendor go build

install:
	GOPATH=$(GOPATH):$(CWD)/vendor go install


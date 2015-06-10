CWD = $(shell pwd)

all:
	GOPATH=$(GOPATH):$(CWD)/pkg
	go build

install:
	GOPATH=$(GOPATH):$(CWD)/pkg
	go install


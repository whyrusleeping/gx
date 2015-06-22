CWD = $(shell pwd)

all:
	GOPATH=$(GOPATH):$(CWD)/vendor
	echo $(GOPATH)
	go build

install:
	GOPATH=$(GOPATH):$(CWD)/vendor
	go install


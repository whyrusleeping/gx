# Run tests
#
# Copyright (c) 2014 Christian Couder
# MIT Licensed; see the LICENSE file in this repository.
#

# NOTE: run with TEST_VERBOSE=1 for verbose sharness tests.

T = $(sort $(wildcard t[0-9][0-9][0-9][0-9]-*.sh))
BINS = bin/gx bin/ipfs bin/go-sleep bin/pollEndpoint
SHARNESS = lib/sharness/sharness.sh

# User might want to override those on the command line
GOFLAGS =

all: aggregate

clean: clean-test-results
	@echo "*** $@ ***"
	-rm -rf bin/ipfs

clean-test-results:
	@echo "*** $@ ***"
	-rm -rf test-results

$(T): clean-test-results deps
	@echo "*** $@ ***"
	./$@

aggregate: clean-test-results $(T)
	@echo "*** $@ ***"
	lib/test-aggregate-results.sh

deps: $(SHARNESS) $(BINS) 

$(SHARNESS): FORCE
	@echo "*** checking $@ ***"
	lib/install-sharness.sh

bin/gx:
	go install github.com/whyrusleeping/gx

bin/ipfs:
	mkdir -p bin
	go get github.com/ipfs/ipfs-update
	ipfs-update fetch v0.4.0-dev --output=bin/ipfs

bin/go-sleep:
	go get -d github.com/chriscool/go-sleep
	go build -o bin/go-sleep github.com/chriscool/go-sleep

bin/pollEndpoint:
	go get -d github.com/whyrusleeping/pollEndpoint
	go build -o bin/pollEndpoint github.com/whyrusleeping/pollEndpoint

race:
	make GOFLAGS=-race all

.PHONY: all clean $(T) aggregate FORCE

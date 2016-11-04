# Minimum version numbers for software required to build gx
GX_MIN_GO_VERSION = 1.7

# The default target of this Makefile is...
all::

go_check:
	@bin/check_go_version $(GX_MIN_GO_VERSION)

all:: go_check
	go build

install: go_check
	go install

test:
	cd tests && make

deps: go_check
	go get .

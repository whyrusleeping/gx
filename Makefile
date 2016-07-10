all:
	go build

install:
	go install

test:
	cd tests && make

deps:
	go get .

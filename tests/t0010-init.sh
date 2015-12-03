#!/bin/sh
#
# Copyright (c) 2015 Jeromy Johnson
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="test package init"

. lib/test-lib.sh

test_expect_success "setup test package" '
	mkdir mypkg &&
	cd mypkg &&
	gx init --lang=none
'

test_expect_success "package.json has right values" '
	NAME=$(jq -r .name package.json) &&
	PKGLANG=$(jq -r .language package.json)
	PKGVERS=$(jq -r .version package.json)
'

test_expect_success "values look correct" '
	test $NAME = "mypkg" &&
	test $PKGLANG = "none" &&
	test $PKGLANG = "0.0.0"
'

test_expect_success "publish package works" '
	gx publish > pub_out
'

test_expect_success "publish output looks good" '
	echo "package mypkg published with hash: QmZSPuqvKeAQar8qHZK9LuMqmT2zvaJw6YQpxXMyce1GU7" > expected &&
	test_cmp expected pub_out
'

test_done

#!/bin/sh
#
# Copyright (c) 2015 Jeromy Johnson
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="test package init"

. lib/test-lib.sh

test_expect_success "setup test package" '
	mkdir mypkg &&
	(cd mypkg && gx init --lang=none)
'

test_expect_success "package.json has right values" '
	NAME=$(jq -r .name mypkg/package.json) &&
	PKGLANG=$(jq -r .language mypkg/package.json)
	PKGVERS=$(jq -r .version mypkg/package.json)
'

test_expect_success "values look correct" '
	test $NAME = "mypkg" &&
	test $PKGLANG = "none" &&
	test $PKGVERS = "0.0.0"
'

test_expect_success "publish package works" '
	pkg_run mypkg gx publish > pub_out
'

test_expect_success "publish output looks good" '
	echo "package mypkg published with hash: QmPx826U5SrMXiuHQbFzBhsuzJqANGf66UpKPK5dKb4z4Y" > expected &&
	test_cmp expected pub_out
'

test_expect_success "publish package second time succeeds" '
	pkg_run mypkg gx publish > pub_out2
'

test_expect_success "publish output is the same on second publish" '
	test_cmp expected pub_out2
'

test_done

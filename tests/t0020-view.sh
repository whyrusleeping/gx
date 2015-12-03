#!/bin/sh
#
# Copyright (c) 2015 Jeromy Johnson
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="test gx view"

. lib/test-lib.sh

test_expect_success "setup test package" '
	mkdir mypkg &&
	cd mypkg &&
	gx init --lang=none
'

test_expect_success "gx view . succeeds" '
	gx view . | jq -S . > full_out
'

test_expect_success "output looks good" '
	cat package.json | jq -S . > sorted_pkg &&
	test_cmp sorted_pkg full_out
'

test_done

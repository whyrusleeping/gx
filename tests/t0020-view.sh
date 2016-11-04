#!/bin/sh
#
# Copyright (c) 2015 Jeromy Johnson
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="test gx view"

. lib/test-lib.sh

test_init_ipfs
test_launch_ipfs_daemon

test_expect_success "setup test package" '
	mkdir mypkg &&
	cd mypkg &&
	gx init --lang=none
'

test_expect_success "gx view . succeeds" '
	gx view . > gx_out &&
	jq -S . gx_out > full_out
'

test_expect_success "output looks good" '
	jq -S . package.json > sorted_pkg &&
	test_cmp sorted_pkg full_out
'

test_expect_success "gx view individual field works" '
	gx view .language > lang_out
'

test_expect_success "gx view individual field works" '
	echo "none" > lang_exp &&
	test_cmp lang_exp lang_out
'

test_expect_success "gx set language field works" '
	gx set .language testLang
'

test_expect_success "gx view language field still works" '
	echo testLang >expected &&
	gx view .language >actual &&
	test_cmp expected actual
'

test_expect_success "gx set license field works" '
	gx set .license MIT
'

test_expect_success "gx view license field works" '
	echo MIT >expected &&
	gx view .license >actual &&
	test_cmp expected actual
'

test_kill_ipfs_daemon

test_done

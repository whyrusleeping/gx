#!/bin/sh
#
# Copyright (c) 2015 Jeromy Johnson
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="test package importing"

. lib/test-lib.sh

test_expect_success "setup test packages" '
	make_package a none
	make_package b none
	make_package c none
'

test_expect_success "publish the packages a and b" '
	pkgA=$(publish_package a) &&
	pkgB=$(publish_package b)
'

test_expect_success

test_done

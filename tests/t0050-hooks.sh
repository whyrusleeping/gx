#!/bin/sh
#
# Copyright (c) 2016 Jeromy Johnson
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="test gx hooks"

. lib/test-lib.sh


test_init_ipfs
test_launch_ipfs_daemon

test_expect_success "init a package" '
	mkdir a &&
	pkg_run a gx init --lang=test 2> init_out
'

test_expect_success "post init was run" '
	echo "HOOK RUN: post-init $(pwd)/a" > init_exp &&
	test_cmp init_exp init_out
'

test_expect_success "publish a package" '
	pkg_hash=$(publish_package a 2> publish_out)
'

test_expect_success "pre and post publish hooks ran" '
	echo "HOOK RUN: pre-publish " > publish_exp &&
	echo "HOOK RUN: post-publish $pkg_hash" >> publish_exp  &&
	test_cmp publish_exp publish_out
'

test_expect_success "create another package" '
	make_package b
'

test_expect_success "import a from b" '
	pkg_run b gx import $pkg_hash 2> import_out
'

test_expect_success "output looks good" '
	echo "HOOK RUN: post-install vendor/gx/ipfs/$pkg_hash" > import_exp &&
	echo "HOOK RUN: post-import $pkg_hash" >> import_exp &&
	test_cmp import_exp import_out
'

test_expect_success "create another package" '
	make_package c
'

test_expect_success "import a globally from c" '
	pkg_run c gx import --global $pkg_hash 2> import_out
'

test_expect_success "output looks good" '
	echo "HOOK RUN: post-install vendor/gx/ipfs/$pkg_hash --global" > import_exp &&
	echo "HOOK RUN: post-import $pkg_hash" >> import_exp &&
	test_cmp import_exp import_out
'

test_kill_ipfs_daemon

test_done

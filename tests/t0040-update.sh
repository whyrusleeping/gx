#!/bin/sh
#
# Copyright (c) 2016 Jeromy Johnson
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="test package updating"

. lib/test-lib.sh

check_package_import() {
	pkg=$1
	imphash=$2
	name=$3

	test_expect_success "dir was created" '
		stat $pkg/vendor/gx/ipfs/$imphash > /dev/null
	'

	test_expect_success "dep set in package.json" '
		jq -r ".gxDependencies[] | select(.hash == \"$imphash\") | .name" $pkg/package.json > name
	'

	test_expect_success "name looks good" '
		echo "$name" > exp_name &&
		test_cmp exp_name name
	'
}

test_init_ipfs
test_launch_ipfs_daemon

# make a tree like this, then update C
# A
# |--B
# C  |
#    C
#

test_expect_success "setup test packages" '
	make_package a none
	make_package b none
	make_package c none
'

test_expect_success "publish package c" '
	pkgC=$(publish_package c)
'

test_expect_success "import package c from a succeeds" '
	pkg_run a gx import $pkgC
'

check_package_import a $pkgC c

test_expect_success "import package c from b succeeds" '
	pkg_run b gx import $pkgC
'

check_package_import b $pkgC c

test_expect_success "publish package b suceeds" '
	pkgB=$(publish_package b)
'

test_expect_success "import package b succeeds" '
	pkg_run a gx import $pkgB
'

check_package_import a $pkgB b

test_expect_success "change package c" '
	echo "test" > c/README.md &&
	pkg_run c gx version v1.2.0 &&
	pkgCnew=$(publish_package c)
'

test_expect_success "should be a different hash" '
	test "$pkgC" != "$pkgCnew"
'

test_expect_success "updating package c works" '
	pkg_run a gx update c $pkgCnew > update_out
'

test_expect_success "update printed correct warning" '
	grep "installing package: c-1.2.0" update_out &&
	grep "dep b also imports c ($pkgC)" update_out
'

test_expect_success "version was updated in package.json" '
	pkg_run a gx view c .version > vers_out
'

test_expect_success "version output looks good" '
	echo "1.2.0" > vers_exp &&
	test_cmp vers_exp vers_out
'

check_package_import a $pkgCnew c

test_kill_ipfs_daemon

test_done

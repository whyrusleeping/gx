#!/bin/sh
#
# Copyright (c) 2016 Jeromy Johnson
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="test package init"

. lib/test-lib.sh

test_init_ipfs
test_launch_ipfs_daemon

pkg_hash="QmXmwLTAxcfSeAXnq6KVsRPTjsJBAvuYnhqqLUrnUyTVNh"

test_expect_success "setup test package" '
	which gx &&
	mkdir mypkg &&
	echo "{\"User\":{\"Name\":\"gxguy\"}}" > mypkg/.gxrc &&
	(cd mypkg && gx init --lang=none)
'

set_package_field() {
	local field="$1"
	local val="$2"
	cat mypkg/package.json | jq "$field = $val" > temp &&
	mv temp mypkg/package.json
	return $?
}

test_expect_success "add a field to the package.json" '
	set_package_field .cats "\"awesome\""
'

test_expect_success "update version" '
	(cd mypkg && gx version 0.4.9)
'

test_expect_success "extra config val remains" '
	cat mypkg/package.json | jq -e .cats > out
'

test_expect_success "output looks good" '
	echo "\"awesome\"" > exp &&
	test_cmp exp out
'

test_kill_ipfs_daemon

test_done

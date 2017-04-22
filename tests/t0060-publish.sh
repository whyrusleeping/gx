#!/bin/sh
#
# Copyright (c) 2017 Jeromy Johnson
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="test gx publish"

. lib/test-lib.sh

test_expect_success "setup test package" '
  mkdir mypkg &&
  cd mypkg &&
  gx init --lang=none
'

test_expect_success "requires ipfs daemon running" '
  IPFS_API=/ip4/127.0.0.1/tcp/12345 gx publish > pub_out 2>&1
  test_should_contain "ipfs daemon isn'"'"'t running" pub_out

  IPFS_API=/ip4/127.0.0.1/tcp/12345 gx release minor > rel_out 2>&1
  test_should_contain "ipfs daemon isn'"'"'t running" rel_out
'

test_done

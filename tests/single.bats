#!/usr/bin/env bats

load shared.bash

@test "mke single --test" {
	run footloose ssh --config $footlooseconfig root@$node0 "mke single --test"
	[ "$status" -eq 0 ]
}

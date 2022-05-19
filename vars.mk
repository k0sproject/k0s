#!/usr/bin/env -S make --no-print-directory -s -f

.PHONY: help
help:
	echo 'Query Makefile variables from scripts. Use like so:' >&2
	echo ' ./vars.mk go_version' >&2
	echo ' ./vars.mk FROM=docs python_version' >&2
	exit 1

FROM := embedded-bins
include $(FROM)/Makefile.variables

# https://stackoverflow.com/a/38803814
.PHONY: .all_phony
.all_phony:

%: .all_phony
	@[ '$(origin $@)' != file ] || echo '$($@)'

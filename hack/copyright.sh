#!/bin/sh

# SPDX-FileCopyrightText: 2023 k0s authors
# SPDX-License-Identifier: Apache-2.0

set -eu

RESULT=0
FIX=${FIX:=n}

get_year(){
    # The -1 has to be added because if a file has been added, removed and added
    # again we'll get two lines with both dates. We only pick the newest one.
    YEAR=$(TZ=UTC git log --follow --find-copies=90% -1 --diff-filter=A --pretty=format:%ad --date=format:%Y -- "$1")
    if [ -z "$YEAR" ]; then
        YEAR=$(TZ=UTC date +%Y)
	    echo "WARN: $1 doesn't seem to be committed in the repo, assuming $YEAR" 1>&2
    fi
    echo "$YEAR"
}

has_basic_copyright(){
	FILE=$1
	grep -q -F "SPDX-FileCopyrightText: k0s authors" "$FILE"
}

has_date_copyright(){
	DATE=$1
	FILE=$2
	grep -q -F "SPDX-FileCopyrightText: $DATE k0s authors" "$FILE"
}

# Deliberately do not search in docs as the date of the matches for the
# Copyright notice aren't related to the date of the document.
for i in $(find cmd hack internal inttest pkg static -type f -name '*.go' -not -name 'zz_generated*'); do
    case "$i" in
    pkg/client/clientset/*)
        if ! has_basic_copyright "$i"; then
          echo "ERROR: $i doesn't have a proper copyright notice" 1>&2
          RESULT=1
        fi
        ;;

    *)
        DATE=$(get_year "$i")

        # codegen gets the header from a static file, so instead we'll replace it every time.
        # Also fix every file if FIX=y
        if [ "$FIX" = 'y' ]; then
          sed -i.tmp -e "s/SPDX-FileCopyrightText: [0-9][0-9][0-9][0-9] k0s authors/SPDX-FileCopyrightText: $DATE k0s authors/" -- "$i" && rm -f "$i".tmp
        fi

        if ! has_date_copyright "$DATE" "$i"; then
          echo "ERROR: $i doesn't have a proper copyright notice. Expected $DATE" 1>&2
          RESULT=1
        fi
        ;;
    esac
done

if [ "$RESULT" != "0" ]; then
    if [ "$FIX" = 'y' ]; then
        echo "hack/copyright.sh can't fix the problem automatically. Manual intervention is required"
    else
        echo "Retry running the script with FIX=y hack/copyright.sh to see if can be fixed automatically"
    fi
fi

exit $RESULT

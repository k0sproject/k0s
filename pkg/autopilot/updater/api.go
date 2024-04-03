// Copyright 2021 k0s authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package updater

import (
	"strconv"
	"strings"

	"github.com/Masterminds/semver/v3"
)

type Update struct {
	Version      Version      `yaml:"version"`
	DownloadURLs DownloadURLs `yaml:"downloadURLs"`
}

// DownloadURLs is a mapping from os-arch to download URLs
type DownloadURLs map[string]map[string]string

type Version string

type Channel struct {
	Name     string    `yaml:"name"`
	Versions []Version `yaml:"versions"`
}

// The version comparison stuff is mostly copy-pasted from semver lib with the addition of dealing with
// build tag identifiers as we use those in k0s versioning scheme.

// Compare compares this version to another one. It returns -1, 0, or 1 if
// the version smaller, equal, or larger than the other version.
//
// Versions are compared by X.Y.Z. Build metadata is ignored. Prerelease is
// lower than the version without a prerelease. Compare always takes into account
// prereleases. If you want to work with ranges using typical range syntaxes that
// skip prereleases if the range is not looking for them use constraints.
func (v Version) Compare(o string) int {
	version, _ := semver.NewVersion(string(v))
	otherVersion, _ := semver.NewVersion(o)

	// Compare the major, minor, and patch version for differences. If a
	// difference is found return the comparison.
	if d := compareSegment(version.Major(), otherVersion.Major()); d != 0 {
		return d
	}
	if d := compareSegment(version.Minor(), otherVersion.Minor()); d != 0 {
		return d
	}
	if d := compareSegment(version.Patch(), otherVersion.Patch()); d != 0 {
		return d
	}

	// At this point the major, minor, and patch versions are the same.
	// As this is k0s related, we need to also look at the build tag
	if d := comparePrerelease(version.Metadata(), otherVersion.Metadata()); d != 0 {
		return d
	}

	// Additional logic for pre
	ps := version.Prerelease()
	po := otherVersion.Prerelease()

	if ps == "" && po == "" {
		return 0
	}
	if ps == "" {
		return 1
	}
	if po == "" {
		return -1
	}

	return comparePrerelease(ps, po)
}

func compareSegment(v, o uint64) int {
	if v < o {
		return -1
	}
	if v > o {
		return 1
	}

	return 0
}

func comparePrerelease(v, o string) int {

	// split the prelease versions by their part. The separator, per the spec,
	// is a .
	sparts := strings.Split(v, ".")
	oparts := strings.Split(o, ".")

	// Find the longer length of the parts to know how many loop iterations to
	// go through.
	slen := len(sparts)
	olen := len(oparts)

	l := slen
	if olen > slen {
		l = olen
	}

	// Iterate over each part of the prereleases to compare the differences.
	for i := 0; i < l; i++ {
		// Since the lentgh of the parts can be different we need to create
		// a placeholder. This is to avoid out of bounds issues.
		stemp := ""
		if i < slen {
			stemp = sparts[i]
		}

		otemp := ""
		if i < olen {
			otemp = oparts[i]
		}

		d := comparePrePart(stemp, otemp)
		if d != 0 {
			return d
		}
	}

	// Reaching here means two versions are of equal value but have different
	// metadata (the part following a +). They are not identical in string form
	// but the version comparison finds them to be equal.
	return 0
}

func comparePrePart(s, o string) int {
	// Fastpath if they are equal
	if s == o {
		return 0
	}

	// When s or o are empty we can use the other in an attempt to determine
	// the response.
	if s == "" {
		if o != "" {
			return -1
		}
		return 1
	}

	if o == "" {
		if s != "" {
			return 1
		}
		return -1
	}

	// When comparing strings "99" is greater than "103". To handle
	// cases like this we need to detect numbers and compare them. According
	// to the semver spec, numbers are always positive. If there is a - at the
	// start like -99 this is to be evaluated as an alphanum. numbers always
	// have precedence over alphanum. Parsing as Uints because negative numbers
	// are ignored.

	oi, n1 := strconv.ParseUint(o, 10, 64)
	si, n2 := strconv.ParseUint(s, 10, 64)

	// The case where both are strings compare the strings
	if n1 != nil && n2 != nil {
		if s > o {
			return 1
		}
		return -1
	} else if n1 != nil {
		// o is a string and s is a number
		return -1
	} else if n2 != nil {
		// s is a string and o is a number
		return 1
	}
	// Both are numbers
	if si > oi {
		return 1
	}
	return -1

}

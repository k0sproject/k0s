/*
Copyright 2024 k0s authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package http_test

import (
	"context"
	"io"
	"net/http"
	"path"
	"testing"

	internalhttp "github.com/k0sproject/k0s/internal/http"

	"github.com/stretchr/testify/assert"
)

func TestDownload_ContentDisposition(t *testing.T) {
	// Test cases taken from http://test.greenbytes.de/tech/tc2231/.
	// Note that some of those are not passing as is, because mime.ParseMimeType
	// is a bit lax on input validation. The outcomes will be overridden later on.
	tests := map[string]*struct {
		header   string // the Content-Disposition header to be sent by the server
		fileName string // the expected file name
	}{
		// This should be equivalent to not including the header at all.
		"inlonly": {`inline`, "inlonly"},

		// This is invalid syntax, thus the header should be ignored.
		"inlonlyquoted": {`"inline"`, "inlonlyquoted"},

		// 'inline', specifying a filename of foo.html
		// Some UAs use this filename in a subsequent "save" operation.
		"inlwithasciifilename": {`inline; filename="foo.html"`, "foo.html"},

		// Test cases which are not applicable here:
		// - inlwithfnattach
		// - inlwithasciifilenamepdf

		// 'attachment' only
		// UA should offer to download the resource.
		"attonly": {`attachment`, "attonly"},

		// 'attachment' only, using double quotes
		// This is invalid syntax, thus the header should be ignored.
		"attonlyquoted": {`"attachment"`, "attonlyquoted"},

		// Test cases which are not applicable here:
		// - attonly403

		// 'ATTACHMENT' only
		// UA should offer to download the resource.
		"attonlyucase": {`ATTACHMENT`, "attonlyucase"},

		// 'attachment', specifying a filename of foo.html
		// UA should offer to download the resource as foo.html.
		"attwithasciifilename": {`attachment; filename="foo.html"`, "foo.html"},

		// 'attachment', specifying a filename of 0000000000111111111122222 (25 characters)
		"attwithasciifilename25": {`attachment; filename="0000000000111111111122222"`, "0000000000111111111122222"},

		// 'attachment', specifying a filename of 00000000001111111111222222222233333 (35 characters)
		"attwithasciifilename35": {`attachment; filename="00000000001111111111222222222233333"`, "00000000001111111111222222222233333"},

		// 'attachment', specifying a filename of f\oo.html (the first 'o' being escaped)
		// UA should offer to download the resource as foo.html.
		"attwithasciifnescapedchar": {`attachment; filename="f\oo.html"`, "foo.html"},

		// 'attachment', specifying a filename of \"quoting\" tested.html (using double quotes around "quoting" to test... quoting)
		// UA should offer to download the resource as something like '"quoting" tested.html'
		// (stripping the quotes may be ok for security reasons, but getting confused by them is not).
		"attwithasciifnescapedquote": {`attachment; filename="\"quoting\" tested.html"`, `_quoting_ tested.html`},

		// 'attachment', specifying a filename of Here's a semicolon;.html - this checks for proper parsing for parameters.
		"attwithquotedsemicolon": {`attachment; filename="Here's a semicolon;.html"`, "Here's a semicolon;.html"},

		// 'attachment', specifying a filename of foo.html and an extension parameter "foo" which should be ignored (see Section 4.4 of RFC 6266.).
		// UA should offer to download the resource as foo.html.
		"attwithfilenameandextparam": {`attachment; foo="bar"; filename="foo.html"`, "foo.html"},

		// 'attachment', specifying a filename of foo.html and an extension parameter "foo" which should be ignored (see Section 4.4 of RFC 6266.).
		// In comparison to attwithfilenameandextparam, the extension parameter actually uses backslash-escapes.
		// This tests whether the UA properly skips the parameter.
		// UA should offer to download the resource as foo.html.
		"attwithfilenameandextparamescaped": {`attachment; foo="\"\\";filename="foo.html"`, "foo.html"},

		// 'attachment', specifying a filename of foo.html
		// UA should offer to download the resource as foo.html.
		"attwithasciifilenameucase": {`attachment; FILENAME="foo.html"`, "foo.html"},

		// 'attachment', specifying a filename of foo.html using a token instead of a quoted-string (according to RFC 2045).
		// Note that was invalid according to Section 19.5.1 of RFC 2616.
		"attwithasciifilenamenq": {`attachment; filename=foo.html`, "foo.html"},

		// 'attachment', specifying a filename of foo,bar.html using a comma despite using token syntax.
		"attwithtokfncommanq": {`attachment; filename=foo,bar.html`, "attwithtokfncommanq"},

		// 'attachment', specifying a filename of foo.html using a token instead of a quoted-string, and adding a trailing semicolon.
		// The trailing semicolon makes the header field value syntactically
		// incorrect, as no other parameter follows. Thus the header field
		// should be ignored.
		"attwithasciifilenamenqs": {`attachment; filename=foo.html ;`, "attwithasciifilenamenqs"},

		//  'attachment', specifying a filename of foo, but including an empty parameter.
		// The empty parameter makes the header field value syntactically
		// incorrect, as no other parameter follows. Thus the header field
		// should be ignored.
		"attemptyparam": {`attachment; ;filename=foo`, "attemptyparam"},

		// 'attachment', specifying a filename of foo bar.html without using quoting.
		// This is invalid. "token" does not allow whitespace.
		"attwithasciifilenamenqws": {`attachment; filename=foo bar.html`, "attwithasciifilenamenqws"},

		// 'attachment', specifying a filename of 'foo.bar' using single quotes.
		"attwithfntokensq": {`attachment; filename='foo.bar'`, "'foo.bar'"},

		// 'attachment', specifying a filename of foo-ä.html, using UTF-8
		// encoding. UA should offer to download the resource as "foo-Ã¤.html".
		// Displaying "foo-ä.html" instead indicates that the UA tried to be
		// smart by detecting something that happens to look like UTF-8.
		"attwithutf8fnplain": {`attachment; filename="foo-ä.html"`, "foo-Ã¤.html"},

		// 'attachment', specifying a filename of foo-%41.html
		// UA should offer to download the resource as "foo-%41.html".
		// Displaying "foo-A.html" instead would indicate that the UA has
		// attempted to percent-decode the parameter.
		"attwithfnrawpctenca": {`attachment; filename="foo-%41.html"`, "foo-%41.html"},

		// 'attachment', specifying a filename of 50%.html
		// UA should offer to download the resource as "50%.html". This tests
		// how UAs that fails at attwithfnrawpctenca handle "%" characters that
		// do not start a "% hexdig hexdig" sequence.
		"attwithfnusingpct": {`attachment; filename="50%.html"`, "50%.html"},

		// 'attachment', specifying a filename of foo-%41.html, using an escape character
		// This tests whether adding an escape character inside a %xx sequence
		// can be used to disable the non-conformant %xx-unescaping
		"attwithfnrawpctencaq": {`attachment; filename="foo-%\41.html"`, "foo-%41.html"},

		// 'attachment', specifying a name parameter of foo-%41.html.
		// This test was added to observe the behavior of the (unspecified)
		// treatment of "name" as synonym for "filename"; see Ned Freed's
		// summary where this comes from in MIME messages. Should be treated as
		// extension parameter, therefore almost any behavior is acceptable.
		"attwithnamepct": {`attachment; name="foo-%41.html"`, "attwithnamepct"},

		// 'attachment', specifying a filename parameter of ä-%41.html.
		// This test was added to observe the behavior when non-ASCII characters
		// and percent-hexdig sequences are combined.
		"attwithfilenamepctandiso": {"attachment; filename=\"\xe4-%41.html\"", "ä-%41.html"},

		// 'attachment', specifying a filename of foo-%c3%a4-%e2%82%ac.html, using raw percent encoded UTF-8 to represent foo-ä-€.html
		// UA should offer to download the resource as
		// "foo-%c3%a4-%e2%82%ac.html". Displaying "foo-ä-€.html" instead would
		// indicate that the UA has attempted to percent-decode the parameter
		// (using UTF-8). Displaying something else would indicate that the UA
		// tried to percent-decode, but used a different encoding.
		"attwithfnrawpctenclong": {`attachment; filename="foo-%c3%a4-%e2%82%ac.html"`, "foo-%c3%a4-%e2%82%ac.html"},

		// 'attachment', specifying a filename of foo.html, with one blank space before the equals character.
		// UA should offer to download the resource as "foo.html".
		"attwithasciifilenamews1": {`attachment; filename ="foo.html"`, "foo.html"},

		// 'attachment', specifying two filename parameters. This is invalid syntax.
		"attwith2filenames": {`attachment; filename="foo.html"; filename="bar.html"`, "attwith2filenames"},

		// 'attachment', specifying a filename of foo[1](2).html, but missing the quotes.
		// Also, "[", "]", "(" and ")" are not allowed in the HTTP token production.
		// This is invalid according to Section 19.5.1 of RFC 2616 and RFC 6266,
		// so UAs should ignore it.
		"attfnbrokentoken": {`attachment; filename=foo[1](2).html`, "attfnbrokentoken"},

		// 'attachment', specifying a filename of foo-ä.html, using ISO-8859-1, but missing the quotes.
		// This is invalid, as the umlaut is not a valid token character, so UAs
		// should ignore it.
		"attfnbrokentokeniso": {"attachment; filename=foo-\xe4.html", "attfnbrokentokeniso"},

		// 'attachment', specifying a filename of foo-Ã¤.html (which happens to be foo-ä.html using UTF-8 encoding) but missing the quotes.
		// This is invalid, as the umlaut is not a valid token character, so UAs
		// should ignore it.
		"attfnbrokentokenutf": {`attachment; filename=foo-ä.html`, "attfnbrokentokenutf"},

		// Disposition type missing, filename specified.
		// This is invalid, so UAs should ignore it.
		"attmissingdisposition": {`filename=foo.html`, "attmissingdisposition"},

		// Disposition type missing, filename specified after extension parameter.
		// This is invalid, so UAs should ignore it.
		"attmissingdisposition2": {`x=y; filename=foo.html`, "attmissingdisposition2"},

		// Disposition type missing, filename "qux". Can it be more broken? (Probably)
		// This is invalid, so UAs should ignore it.
		"attmissingdisposition3": {`"foo; filename=bar;baz"; filename=qux`, "attmissingdisposition3"},

		// Disposition type missing, two filenames specified separated by a comma
		// This is syntactically equivalent to have two instances of the header with one filename parameter each.
		// This is invalid, so UAs should ignore it.
		"attmissingdisposition4": {`filename=foo.html, filename=bar.html`, "attmissingdisposition4"},

		// Disposition type missing (but delimiter present), filename specified.
		// This is invalid, so UAs should ignore it.
		"emptydisposition": {`; filename=foo.html`, "emptydisposition"},

		// Header field value starts with a colon.
		// This is invalid, so UAs should ignore it.
		"doublecolon": {`: inline; attachment; filename=foo.html`, "doublecolon"},

		// Both disposition types specified.
		// This is invalid, so UAs should ignore it.
		"attandinline": {`inline; attachment; filename=foo.html`, "attandinline"},

		// Both disposition types specified.
		// This is invalid, so UAs should ignore it.
		"attandinline2": {`attachment; inline; filename=foo.html`, "attandinline2"},

		// 'attachment', specifying a filename parameter that is broken (quoted-string followed by more characters).
		// This is invalid, so UAs should ignore it.
		"attbrokenquotedfn": {`attachment; filename="foo.html".txt`, "attbrokenquotedfn"},

		// 'attachment', specifying a filename parameter that is broken (missing ending double quote).
		// This is invalid, so UAs should ignore it.
		"attbrokenquotedfn2": {`attachment; filename="bar`, "attbrokenquotedfn2"},

		// 'attachment', specifying a filename parameter that is broken (disallowed characters in token syntax).
		// This is invalid, so UAs should ignore it.
		"attbrokenquotedfn3": {`attachment; filename=foo"bar;baz"qux`, "attbrokenquotedfn3"},

		// 'attachment', two comma-separated instances of the header field.
		// As Content-Disposition doesn't use a list-style syntax, this is
		// invalid syntax and, according to RFC 2616, Section 4.2, roughly
		// equivalent to having two separate header field instances. This is
		// invalid, so UAs should ignore it.
		"attmultinstances": {`attachment; filename=foo.html, attachment; filename=bar.html`, "attmultinstances"},

		// Uses two parameters, but the mandatory delimiter ";" is missing.
		// This is invalid, so UAs should ignore it.
		"attmissingdelim": {`attachment; foo=foo filename=bar`, "attmissingdelim"},

		// Uses two parameters, but the mandatory delimiter ";" is missing.
		// This is invalid, so UAs should ignore it.
		"attmissingdelim2": {`attachment; filename=bar foo=foo`, "attmissingdelim2"},

		// ";" missing between disposition type and filename parameter.
		// This is invalid, so UAs should ignore it.
		"attmissingdelim3": {`attachment filename=bar`, "attmissingdelim3"},

		// filename parameter and disposition type reversed.
		// This is invalid, so UAs should ignore it.
		"attreversed": {`filename=foo.html; attachment`, "attreversed"},

		// 'attachment', specifying an "xfilename" parameter.
		// Should be treated as unnamed attachment.
		"attconfusedparam": {`attachment; xfilename=foo.html`, "attconfusedparam"},

		// 'attachment', specifying an absolute filename in the filesystem root.
		// Either ignore the filename altogether, or discard the path information.
		"attabspath": {`attachment; filename="/foo.html"`, "_foo.html"},

		// 'attachment', specifying an absolute filename in the filesystem root.
		// Either ignore the filename altogether, or discard the path
		// information. Note that test results under Operating Systems other
		// than Windows vary; apparently some UAs consider the backslash a
		// legitimate filename character.
		"attabspathwin": {`attachment; filename="\\foo.html"`, "_foo.html"},

		// ... Leaving out the whole "Additional Parameters" section ...

		// 'foobar' only
		// This should be equivalent to using "attachment".
		"dispext": {`foobar`, "dispext"},

		// 'attachment', with no filename parameter
		"dispextbadfn": {`attachment; example="filename=example.txt"`, "dispextbadfn"},

		// 'attachment', specifying a filename of foo-ä.html, using RFC2231/5987 encoded ISO-8859-1
		// UA should offer to download the resource as "foo-ä.html".
		"attwithisofn2231iso": {`attachment; filename*=iso-8859-1''foo-%E4.html`, "foo-ä.html"},

		// 'attachment', specifying a filename of foo-ä-€.html, using RFC2231/5987 encoded UTF-8
		// UA should offer to download the resource as "foo-ä-€.html".
		"attwithfn2231utf8": {`attachment; filename*=UTF-8''foo-%c3%a4-%e2%82%ac.html`, "foo-ä-€.html"},

		// Behavior is undefined in RFC 2231, the charset part is missing, although UTF-8 was used.
		"attwithfn2231noc": {`attachment; filename*=''foo-%c3%a4-%e2%82%ac.html`, "attwithfn2231noc"},

		// 'attachment', specifying a filename of foo-ä.html, using RFC2231
		// encoded UTF-8, but choosing the decomposed form (lowercase a plus
		// COMBINING DIAERESIS) -- on a Windows target system, this should be
		// translated to the preferred Unicode normal form (composed). UA should
		// offer to download the resource as "foo-ä.html".
		"attwithfn2231utf8comp": {`attachment; filename*=UTF-8''foo-a%cc%88.html`, "foo-ä.html"},

		// 'attachment', specifying a filename of foo-ä-€.html, using RFC2231 encoded UTF-8, but declaring ISO-8859-1
		// The octet %82 does not represent a valid ISO-8859-1 code point, so
		// the UA should really ignore the parameter.
		"attwithfn2231utf8-bad": {`attachment; filename*=iso-8859-1''foo-%c3%a4-%e2%82%ac.html`, "attwithfn2231utf8-bad"},

		// 'attachment', specifying a filename of foo-ä.html, using RFC2231 encoded ISO-8859-1, but declaring UTF-8
		// The octet %E4 does not represent a valid UTF-8 octet sequence, so the
		// UA should really ignore the parameter.
		"attwithfn2231iso-bad": {`attachment; filename*=utf-8''foo-%E4.html`, "attwithfn2231iso-bad"},

		// 'attachment', specifying a filename of foo-ä.html, using RFC2231 encoded UTF-8, with whitespace before "*="
		// The parameter is invalid, thus should be ignored.
		"attwithfn2231ws1": {`attachment; filename *=UTF-8''foo-%c3%a4.html`, "attwithfn2231ws1"},

		// 'attachment', specifying a filename of foo-ä.html, using RFC2231 encoded UTF-8, with whitespace after "*="
		// UA should offer to download the resource as "foo-ä.html".
		"attwithfn2231ws2": {`attachment; filename*= UTF-8''foo-%c3%a4.html`, "foo-ä.html"},

		// 'attachment', specifying a filename of foo-ä.html, using RFC2231 encoded UTF-8, with whitespace inside "* ="
		// UA should offer to download the resource as "foo-ä.html".
		"attwithfn2231ws3": {`attachment; filename* =UTF-8''foo-%c3%a4.html`, "foo-ä.html"},

		// 'attachment', specifying a filename of foo-ä.html, using RFC2231 encoded UTF-8, with double quotes around the parameter value.
		// The parameter is invalid, thus should be ignored.
		"attwithfn2231quot": {`attachment; filename*="UTF-8''foo-%c3%a4.html"`, "attwithfn2231quot"},

		// 'attachment', specifying a filename of foo bar.html, using "filename*", but missing character encoding and language.
		// The parameter is invalid, thus should be ignored.
		"attwithfn2231quot2": {`attachment; filename*="foo%20bar.html"`, "attwithfn2231quot2"},

		// 'attachment', specifying a filename of foo-ä.html, using RFC2231 encoded UTF-8, but a single quote is missing.
		// The parameter is invalid, thus should be ignored.
		"attwithfn2231singleqmissing": {`attachment; filename*=UTF-8'foo-%c3%a4.html`, "attwithfn2231singleqmissing"},

		// 'attachment', specifying a filename of foo%, using RFC2231 encoded UTF-8, with a single "%" at the end.
		// The parameter is invalid, thus should be ignored.
		"attwithfn2231nbadpct1": {`attachment; filename*=UTF-8''foo%`, "attwithfn2231nbadpct1"},

		// 'attachment', specifying a filename of f%oo.html, using RFC2231 encoded UTF-8, with a "%" not starting a percent-escape.
		// The parameter is invalid, thus should be ignored.
		"attwithfn2231nbadpct2": {`attachment; filename*=UTF-8''f%oo.html`, "attwithfn2231nbadpct2"},

		// 'attachment', specifying a filename of A-%41.html, using RFC2231 encoded UTF-8.
		"attwithfn2231dpct": {`attachment; filename*=UTF-8''A-%2541.html`, "A-%41.html"},

		// 'attachment', specifying a filename of \foo.html, using RFC2231 encoded UTF-8.
		"attwithfn2231abspathdisguised": {`attachment; filename*=UTF-8''%5cfoo.html`, "_foo.html"},

		// RFC2231 Encoding: Continuations (optional)
	}

	// The mime.ParseMediaType implementation will fail for some of the above
	// test cases, There's no way to fix that besides either fixing it upstream
	// in the Go standard library or implementing the whole stuff by ourselves 🥴
	// Here are the exceptions:

	// This invalid header is simply accepted.
	// This is in line with all the major browsers, it seems.
	// http://test.greenbytes.de/tech/tc2231/#attwithasciifilenamenqs
	tests["attwithasciifilenamenqs"].fileName = "foo.html"

	// This invalid header is simply accepted.
	// Browsers handle this differently ...
	// http://test.greenbytes.de/tech/tc2231/#attwithfn2231quot
	tests["attwithfn2231quot"].fileName = "foo-ä.html"

	// mime.ParseMediaType only supports US-ASCII and UTF-8
	// Doesn't look like that's changing anytime soon ... ¯\_(ツ)_/¯
	tests["attwithisofn2231iso"].fileName = "attwithisofn2231iso"

	// This kind of escape sequence is not recognized.
	tests["attwithasciifnescapedchar"].fileName = "f_oo.html"
	tests["attwithfnrawpctencaq"].fileName = "foo-%_41.html"

	baseURL := startFakeDownloadServer(t, false, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Disposition", tests[path.Base(r.URL.Path)].header)
	}))

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			var fileName string
			err := internalhttp.Download(context.TODO(), baseURL+"/"+name, io.Discard,
				internalhttp.StoreSuggestedRemoteFileNameInto(&fileName),
			)
			assert.NoError(t, err)
			assert.Equal(t, test.fileName, fileName)
		})
	}
}

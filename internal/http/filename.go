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

package http

import (
	"errors"
	"fmt"
	"mime"
	"net/http"
	"path"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/unicode/norm"
)

// Stores the file name suggested by the server in the given pointer.
// Requesting the suggested file name will cause the download to fail if the
// file name detection fails.
func StoreSuggestedRemoteFileNameInto(target *string) DownloadOption {
	return func(opts *downloadOptions) {
		opts.remoteFileNameTarget = target
	}
}

type downloadFileNameOptions struct {
	remoteFileNameTarget *string
}

func (o *downloadFileNameOptions) detectRemoteFileName(resp *http.Response) error {
	if o.remoteFileNameTarget == nil {
		return nil
	}

	name, err := fileNameFor(resp)
	if err != nil {
		return fmt.Errorf("failed to determine file name: %w", err)
	}
	*o.remoteFileNameTarget = name
	return nil
}

func fileNameFor(resp *http.Response) (string, error) {
	var hdrErr error
	cd := resp.Header.Values("Content-Disposition")
	switch len(cd) {
	case 1:
		fileName, hdrErr := fileNameFromContentDisposition(cd[0])
		if hdrErr == nil {
			return fileName, nil
		}
	case 0:
		hdrErr = errors.New("none provided")
	default:
		hdrErr = errors.New("multiple headers")
	}

	fileName, pathErr := fileNameFromRequestPath(resp.Request.URL.Path)
	if pathErr == nil {
		return fileName, nil
	}

	return "", fmt.Errorf("Content-Disposition header failed (%w), request path failed (%w)", hdrErr, pathErr)
}

func fileNameFromContentDisposition(contentDisposition string) (string, error) {
	// RFC 2183: The Content-Disposition Header Field
	// https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Disposition#as_a_response_header_for_the_main_body

	// The header values are in ISO-8859-1, but mime.ParseMediaType doesn't
	// address any character encoding conversions. It will pass on all the bytes
	// as-is. Every multi-byte character needs to be presented as percent
	// escapes anyway, so convert the whole string from ISO-8859-1 to UTF-8.
	utf8, err := charmap.ISO8859_1.NewDecoder().String(contentDisposition)
	if err != nil { // Can this fail at all? ISO-8859-1 to UTF-8 should always be possible.
		return "", err
	}

	_, params, err := mime.ParseMediaType(utf8)
	if err != nil {
		return "", err
	}

	// RFC 2183, section 2.3: The Filename Parameter
	// The standard library's mime package takes care of the RFC 5987 syntax
	// transparently, so all the star-suffixed params are folded into the
	// canonical ones.
	if fileName, ok := params["filename"]; ok {
		return sanitizeFileName(fileName)
	}

	return "", fmt.Errorf("no filename parameter")
}

func fileNameFromRequestPath(requestPath string) (string, error) {
	if requestPath == "" {
		return "", fmt.Errorf("request path is empty")
	}

	fileName := path.Base(requestPath)
	if fileName == "/" {
		return "", fmt.Errorf("request path has no base")
	}

	return sanitizeFileName(fileName)
}

func sanitizeFileName(fileName string) (string, error) {

	// https://www.w3.org/International/questions/qa-bidi-unicode-controls#basedirection
	const (
		lri = '\u2066' // LEFT-TO-RIGHT ISOLATE
		rli = '\u2067' // RIGHT-TO-LEFT ISOLATE
		fsi = '\u2068' // FIRST-STRONG ISOLATE
		lre = '\u202a' // LEFT-TO-RIGHT EMBEDDING
		rle = '\u202b' // RIGHT-TO-LEFT EMBEDDING
		lro = '\u202d' // LEFT-TO-RIGHT OVERRIDE
		rlo = '\u202e' // RIGHT-TO-LEFT OVERRIDE
		pdi = '\u2069' // POP DIRECTIONAL ISOLATE
		pdf = '\u202c' // POP DIRECTIONAL FORMATTING
	)

	// Perform unicode normalization to get path names in the operating system's
	// preferred form. Linux and Windows usually prefer NFC, whereas macOS tends
	// to prefer NFD. Since k0s doesn't run on macOS, it's supposedly fine to
	// stick to NFC. See also below.
	fileName = norm.NFC.String(fileName)

	if len(fileName) > 255 {
		return "", errors.New("too long")
	}

	switch fileName {
	case "":
		return "", errors.New("empty")
	case ".":
		return "", errors.New("dot")
	case "..":
		return "", errors.New("dotdot")
	}

	// https://learn.microsoft.com/en-us/windows/win32/fileio/naming-a-file
	if base, _, _ := strings.Cut(fileName, "."); len(base) < 6 {
		switch strings.ToLower(base) {
		case "con", "prn", "aux", "nul",
			"com0", "com1", "com2", "com3", "com4", "com5", "com6", "com7", "com8", "com9",
			"lpt0", "lpt1", "lpt2", "lpt3", "lpt4", "lpt5", "lpt6", "lpt7", "lpt8", "lpt9",
			"com¹", "com²", "com³", "lpt¹", "lpt²", "lpt³": // this line relies on NFC
			return "", errors.New("reserved name")
		}
	}

	// Now replace all the problematic characters.
	var sanitized strings.Builder
	for pos, size, len := 0, 0, len(fileName); pos < len; pos += size {
		var (
			ch    rune
			valid bool
		)

		ch, size = utf8.DecodeRuneInString(fileName[pos:])
		switch ch {
		case '\\', '/': // path separators
		case '<', '>', '|': // redirections and pipes are not safe for shells and are not allowed in Windows file names
		case ':': // used to specify Windows drive letters and not allowed in Windows file names
		case '"': // used to encapsulate file names in shells and not allowed in Windows file names
		case '?', '*': // shell wildcards and not allowed in Windows file names
			// all of the above get replaced

		case lri, rli, fsi, lre, rle, lro, rlo, pdi, pdf:
			// replace all text direction modifications
			// https://www.freecodecamp.org/news/rtlo-in-hacking/

		case utf8.RuneError:
			if size == 1 { // "if the encoding is invalid, it returns (RuneError, 1)"
				return "", fmt.Errorf("invalid UTF-8 at position %d: 0x%2x", pos, fileName[pos])
			}
			valid = true // It's fine otherwise.

		default:
			switch {
			case unicode.Is(unicode.Other, ch): // control characters
			case pos == 0 && unicode.IsSpace(ch): // leading space
			case pos == len-1 && (ch == '.' || unicode.IsSpace(ch)): // trailing dot/space
				// all of the above get replaced
			default: // legit character
				valid = true
			}
		}

		if valid {
			if sanitized.Len() > 0 {
				sanitized.WriteRune(ch)
			}
		} else {
			if sanitized.Len() < 1 {
				sanitized.Grow(len)
				sanitized.WriteString(fileName[:pos])
			}
			sanitized.WriteByte('_')
		}
	}

	if sanitized.Len() > 0 {
		fileName = sanitized.String()
	}

	return fileName, nil
}

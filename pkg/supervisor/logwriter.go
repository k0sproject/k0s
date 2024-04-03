/*
Copyright 2023 k0s authors

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

package supervisor

import (
	"bytes"
	"unicode/utf8"

	"github.com/sirupsen/logrus"
)

// logWriter implements [io.Writer] by forwarding whole lines to log. In case
// the lines get too long, it logs them in multiple chunks.
//
// This is in contrast to logrus's implementation of io.Writer, which simply
// errors out if the log line gets longer than 64k.
type logWriter struct {
	log     logrus.FieldLogger // receives (possibly chunked) log lines
	buf     []byte             // buffer in which to accumulate chunks; len(buf) determines the chunk length
	len     int                // current buffer length
	chunkNo uint               // current chunk number; 0 means "no chunk"
}

// Write implements [io.Writer].
func (w *logWriter) Write(in []byte) (int, error) {
	w.writeBytes(in)
	return len(in), nil
}

func (w *logWriter) writeBytes(in []byte) {
	// Fill and drain buffer with available data until everything has been consumed.
	for rest := in; len(rest) > 0; {

		n := copy(w.buf[w.len:], rest) // fill buffer with new input data
		rest = rest[n:]                // strip copied input data
		w.len += n                     // increase buffer length accordingly

		// Loop over buffer as long as there are newlines in it
		for off := 0; ; {
			idx := bytes.IndexRune(w.buf[off:w.len], '\n')

			// Discard already logged chunks and break if no newline left
			if idx < 0 {
				if off > 0 {
					w.len = copy(w.buf, w.buf[off:w.len])
				}
				break
			}

			// Strip trailing carriage returns
			line := bytes.TrimRight(w.buf[off:off+idx], "\r")

			if w.chunkNo == 0 {
				w.log.Infof("%s", line)
			} else {
				if len(line) > 0 {
					w.log.WithField("chunk", w.chunkNo+1).Infof("%s", line)
				}
				w.chunkNo = 0
			}

			off += idx + 1 // advance read offset behind the newline
		}

		// Issue a chunked log entry in case the buffer is full
		if w.len == len(w.buf) {
			// Try to chunk at UTF-8 rune boundaries
			len := w.len
			for i := 0; i < utf8.MaxRune && i < w.len; i++ {
				if r, _ := utf8.DecodeLastRune(w.buf[:w.len-i]); r != utf8.RuneError {
					len = len - i
					break
				}
			}

			// Strip trailing carriage returns
			line := bytes.TrimRight(w.buf[:len], "\r")

			w.log.WithField("chunk", w.chunkNo+1).Infof("%s", line)
			w.chunkNo++                      // increase chunk number
			w.len = copy(w.buf, w.buf[len:]) // discard logged bytes
		}
	}
}

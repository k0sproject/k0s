// SPDX-FileCopyrightText: 2023 k0s authors
// SPDX-License-Identifier: Apache-2.0

package log

import (
	"bytes"
	"unicode/utf8"

	"github.com/sirupsen/logrus"
)

// Writer implements [io.Writer] by forwarding whole lines to a logger. In case
// the lines get too long, it logs them in multiple chunks.
//
// This is in contrast to logrus's implementation of io.Writer, which simply
// errors out if the log line gets longer than 64k.
type Writer struct {
	log     *logrus.Entry // receives (possibly chunked) log lines
	level   logrus.Level  // log level used to log lines
	buf     []byte        // buffer in which to accumulate chunks; len(buf) determines the chunk length
	len     int           // current buffer length
	chunkNo uint          // current chunk number; 0 means "no chunk"
}

func NewWriter(log *logrus.Entry, level logrus.Level, chunkLen int) *Writer {
	return &Writer{
		log:   log,
		level: level,
		buf:   make([]byte, chunkLen),
	}
}

// Write implements [io.Writer].
func (w *Writer) Write(in []byte) (int, error) {
	w.writeBytes(in)
	return len(in), nil
}

func (w *Writer) writeBytes(in []byte) {
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
				w.log.Logf(w.level, "%s", line)
			} else {
				if len(line) > 0 {
					w.log.WithField("chunk", w.chunkNo+1).Logf(w.level, "%s", line)
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
					len -= i
					break
				}
			}

			// Strip trailing carriage returns
			line := bytes.TrimRight(w.buf[:len], "\r")

			w.log.WithField("chunk", w.chunkNo+1).Logf(w.level, "%s", line)
			w.chunkNo++                      // increase chunk number
			w.len = copy(w.buf, w.buf[len:]) // discard logged bytes
		}
	}
}

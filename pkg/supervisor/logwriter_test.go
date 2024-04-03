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
	"testing"

	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
)

func TestLogWriter(t *testing.T) {
	type entry struct {
		chunk uint
		msg   string
	}

	for _, test := range []struct {
		name    string
		bufSize int
		in      []string
		out     []entry
	}{
		{"empty_write", 3,
			[]string{""},
			[]entry{}},
		{"single_line", 3,
			[]string{"ab\n"},
			[]entry{{0, "ab"}}},
		{"exact_lines", 3,
			[]string{"abc\n", "def\n"},
			[]entry{{1, "abc"}, {1, "def"}}},
		{"multi_line", 3,
			[]string{"ab\ncd\n"},
			[]entry{{0, "ab"}, {0, "cd"}}},
		{"overlong_lines", 3,
			[]string{"abcd\nef\n"},
			[]entry{{1, "abc"}, {2, "d"}, {0, "ef"}}},
		{"overlong_lines_2", 3,
			[]string{"abcd\ne", "f", "\n"},
			[]entry{{1, "abc"}, {2, "d"}, {0, "ef"}}},
		{"unterminated_consecutive_writes_4", 3,
			[]string{"ab", "cd"},
			[]entry{{1, "abc"}}},
		{"unterminated_consecutive_writes_6", 3,
			[]string{"ab", "cd", "ef"},
			[]entry{{1, "abc"}, {2, "def"}}},
		{"unterminated_consecutive_writes_8", 3,
			[]string{"ab", "cd", "ef", "gh"},
			[]entry{{1, "abc"}, {2, "def"}}},
		{"unterminated_consecutive_writes_10", 3,
			[]string{"ab", "cd", "ef", "gh", "ij"},
			[]entry{{1, "abc"}, {2, "def"}, {3, "ghi"}}},
		{"long_buffer_short_lines", 16,
			[]string{"a\nb\nc\n"},
			[]entry{{0, "a"}, {0, "b"}, {0, "c"}}},
		{"utf8", 26, // would split after the third byte of 🫣
			[]string{"this is four bytes: >>>🫣\n<<<\n"},
			[]entry{{1, "this is four bytes: >>>"}, {2, "🫣"}, {0, "<<<"}}},
		{"strips_carriage_returns", 5,
			[]string{"abc\r\ndef\r\n"},
			[]entry{{0, "abc"}, {0, "def"}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			log, logs := logtest.NewNullLogger()
			underTest := logWriter{log: log, buf: make([]byte, test.bufSize)}

			for _, line := range test.in {
				underTest.writeBytes([]byte(line))
			}

			remaining := logs.AllEntries()

			for i, line := range test.out {
				if !assert.NotEmpty(t, remaining, "Expected additional log entry: %s", line) {
					continue
				}

				chunk, isChunk := remaining[0].Data["chunk"]
				assert.Equal(t, line.chunk != 0, isChunk, "Log entry %d chunk mismatch", i)
				if isChunk {
					assert.Equal(t, line.chunk, chunk, "Log entry %d differs in chunk", i)
				}

				assert.Equal(t, line.msg, remaining[0].Message, "Log entry %d differs in message", i)
				remaining = remaining[1:]
			}

			for _, entry := range remaining {
				assert.Fail(t, "Unexpected log entry", "%s", entry.Message)
			}
		})
	}
}

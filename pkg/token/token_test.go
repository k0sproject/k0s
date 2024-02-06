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

package token

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestGenerate23Base36(t *testing.T) {
	for _, test := range []struct {
		name     string
		expected string
		rng      [][]byte
	}{
		{"high_bit_set", "00000000000000000000001", [][]byte{
			// this is 1 plus the high bit set, which is to be ignored
			{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 128},
		}},
		{"max", "zzzzzzzzzzzzzzzzzzzzzzz", [][]byte{
			// this is 36^23-1, so all z's
			{255, 255, 255, 255, 255, 63, 126, 1, 247, 198, 125, 95, 126, 29, 120, 0},
		}},
		{"skips_overflow", "00000000000000000000001", [][]byte{
			// this is 36^23 which is an overflow -> ask for more bytes
			{0, 0, 0, 0, 0, 64, 126, 1, 247, 198, 125, 95, 126, 29, 120, 0},
			// the second one is a one, once again
			{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 128},
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			rng := new(mockRNG)
			for _, bytes := range test.rng {
				rng.add(bytes)
			}

			data, err := generate23Base36(rng.generate)

			assert.NoError(t, err)
			assert.Equal(t, test.expected, string(data[:]))
			rng.AssertExpectations(t)
		})
	}

	t.Run("forwards_rng_err", func(t *testing.T) {
		rng := new(mockRNG)
		rng.onGenerate().Return(assert.AnError)

		data, err := generate23Base36(rng.generate)

		assert.Nil(t, data)
		assert.Same(t, assert.AnError, err)
		rng.AssertExpectations(t)
	})
}

type mockRNG struct {
	mock.Mock
}

func (m *mockRNG) generate(b []byte) error {
	args := m.Called(b)
	return args.Error(0)
}

func (m *mockRNG) onGenerate() *mock.Call {
	return m.On("generate", mock.AnythingOfType("[]uint8"))
}

func (m *mockRNG) add(data []byte) {
	m.onGenerate().Return(nil).Once().Run(func(args mock.Arguments) {
		arg := args.Get(0).([]byte)
		for i := range arg {
			arg[i] = data[i]
		}
	})
}

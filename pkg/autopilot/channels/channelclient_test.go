// Copyright 2023 k0s authors
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

package channels

import (
	"testing"
)

func TestNewChannelClientChannelURL(t *testing.T) {
	type args struct {
		server  string
		channel string
		token   string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "full URL",
			args: args{
				server:  "https://example.com",
				channel: "foo",
				token:   "",
			},
			want: "https://example.com/foo/index.yaml",
		},
		{
			name: "partial URL",
			args: args{
				server:  "example.com",
				channel: "foo",
				token:   "",
			},
			want: "https://example.com/foo/index.yaml",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewChannelClient(tt.args.server, tt.args.channel, tt.args.token)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewChannelClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got.channelURL != tt.want {
				t.Errorf("NewChannelClient() = %v, want %v", got.channelURL, tt.want)
			}
		})
	}
}

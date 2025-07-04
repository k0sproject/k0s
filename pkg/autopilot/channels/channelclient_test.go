// SPDX-FileCopyrightText: 2023 k0s authors
// SPDX-License-Identifier: Apache-2.0

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

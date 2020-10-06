package util

import "testing"

func TestStringSliceContains(t *testing.T) {
	type args struct {
		strSlice []string
		str      string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "not contains",
			args: args{
				strSlice: []string{"foo", "bar"},
				str:      "foobar",
			},
			want: false,
		},
		{
			name: "contains",
			args: args{
				strSlice: []string{"foo", "bar"},
				str:      "bar",
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StringSliceContains(tt.args.strSlice, tt.args.str); got != tt.want {
				t.Errorf("StringSliceContains() = %v, want %v", got, tt.want)
			}
		})
	}
}

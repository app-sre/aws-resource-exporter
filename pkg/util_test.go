package pkg

import (
	"reflect"
	"testing"
)

func TestWithKeyValue(t *testing.T) {
	type args struct {
		m     map[string]string
		key   string
		value string
	}
	tests := []struct {
		name string
		args args
		want map[string]string
	}{
		{
			name: "Adding a key-value-pair to empty map returns a map with one key-value-pair",
			args: args{
				m:     map[string]string{},
				key:   "new",
				value: "new",
			},
			want: map[string]string{"new": "new"},
		},
		{
			name: "Adding a key-value-pair to existing map returns a new map with an additional key-value-pair",
			args: args{
				m:     map[string]string{"old": "old"},
				key:   "new",
				value: "new",
			},
			want: map[string]string{"old": "old", "new": "new"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := WithKeyValue(tt.args.m, tt.args.key, tt.args.value); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("WithKeyValue() = %v, want %v", got, tt.want)
			}
		})
	}
}

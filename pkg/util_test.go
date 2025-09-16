package pkg

import (
	"log/slog"
	"os"
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

func TestCalculateTotalIPsFromCIDR(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	tests := []struct {
		name        string
		cidrBlock   string
		expectedIPs int64
		expectError bool
	}{
		{
			name:        "Valid /24 subnet",
			cidrBlock:   "10.0.1.0/24",
			expectedIPs: 256,
			expectError: false,
		},
		{
			name:        "Valid /28 subnet",
			cidrBlock:   "10.0.1.0/28",
			expectedIPs: 16,
			expectError: false,
		},
		{
			name:        "Valid /16 subnet",
			cidrBlock:   "10.0.0.0/16",
			expectedIPs: 65536,
			expectError: false,
		},
		{
			name:        "Invalid CIDR format - no slash",
			cidrBlock:   "10.0.1.0",
			expectedIPs: 0,
			expectError: true,
		},
		{
			name:        "Invalid CIDR format - multiple slashes",
			cidrBlock:   "10.0.1.0/24/16",
			expectedIPs: 0,
			expectError: true,
		},
		{
			name:        "Invalid prefix length - non-numeric",
			cidrBlock:   "10.0.1.0/abc",
			expectedIPs: 0,
			expectError: true,
		},
		{
			name:        "Invalid prefix length - too small for AWS",
			cidrBlock:   "10.0.0.0/15",
			expectedIPs: 0,
			expectError: true,
		},
		{
			name:        "Invalid prefix length - too large for AWS",
			cidrBlock:   "10.0.1.0/29",
			expectedIPs: 0,
			expectError: true,
		},
		{
			name:        "Edge case - /16 (largest AWS subnet)",
			cidrBlock:   "172.16.0.0/16",
			expectedIPs: 65536,
			expectError: false,
		},
		{
			name:        "Edge case - /28 (smallest AWS subnet)",
			cidrBlock:   "192.168.1.0/28",
			expectedIPs: 16,
			expectError: false,
		},
		{
			name:        "Invalid prefix length - negative",
			cidrBlock:   "10.0.1.0/-1",
			expectedIPs: 0,
			expectError: true,
		},
		{
			name:        "Invalid prefix length - too large",
			cidrBlock:   "10.0.1.0/33",
			expectedIPs: 0,
			expectError: true,
		},
		{
			name:        "Invalid IP address",
			cidrBlock:   "999.999.999.999/24",
			expectedIPs: 0,
			expectError: true,
		},
		{
			name:        "IPv6 CIDR (should fail AWS validation)",
			cidrBlock:   "2001:db8::/32",
			expectedIPs: 0,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := CalculateTotalIPsFromCIDR(tt.cidrBlock, logger)

			if tt.expectError {
				if err == nil {
					t.Errorf("CalculateTotalIPsFromCIDR() expected error but got none")
				}
				if result != 0 {
					t.Errorf("CalculateTotalIPsFromCIDR() expected 0 IPs when error, got %d", result)
				}
			} else {
				if err != nil {
					t.Errorf("CalculateTotalIPsFromCIDR() unexpected error: %v", err)
				}
				if result != tt.expectedIPs {
					t.Errorf("CalculateTotalIPsFromCIDR() = %d, want %d", result, tt.expectedIPs)
				}
			}
		})
	}
}

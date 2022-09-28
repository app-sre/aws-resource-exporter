package pkg

import (
	"math"
	"reflect"
	"testing"
	"time"
)

func TestGetMetricById(t *testing.T) {
	type args struct {
		mp  *MetricProxy
		key string
	}
	now := time.Now()
	tests := []struct {
		name string
		args args
		want *MetricProxyItem
	}{
		{
			name: "Attempt retrieving value by providing valid id (key exists)",
			args: args{
				mp: &MetricProxy{
					metrics: map[string]*MetricProxyItem{
						"valid": &MetricProxyItem{
							value:        "value",
							ttl:          math.MaxInt32,
							creationTime: now,
						},
					},
				},
				key: "valid",
			},
			want: &MetricProxyItem{
				value:        "value",
				ttl:          math.MaxInt32,
				creationTime: now,
			},
		},
		{
			name: "Attempt retrieving value by providing valid id but ttl expired",
			args: args{
				mp: &MetricProxy{
					metrics: map[string]*MetricProxyItem{
						"expired": &MetricProxyItem{
							value:        100,
							ttl:          1,
							creationTime: now,
						},
					},
				},
				key: "expired",
			},
			want: nil,
		},
	}

	time.Sleep(2 * time.Second) //ensure ttl for second test expires

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, _ := tt.args.mp.GetMetricById(tt.args.key); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetMetricById() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStoreMetricById(t *testing.T) {
	type args struct {
		mp    *MetricProxy
		key   string
		value interface{}
		ttl   int
	}
	tests := []struct {
		name string
		args args
		want *MetricProxyItem
	}{
		{
			name: "Attempt storing new metric by id",
			args: args{
				mp:    NewMetricProxy(),
				key:   "new",
				value: 1,
				ttl:   math.MaxInt32,
			},
			want: &MetricProxyItem{
				value: 1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.args.mp.StoreMetricById(tt.args.key, tt.args.value, tt.args.ttl)
			if got, err := tt.args.mp.GetMetricById(tt.args.key); got.value != tt.want.value || err != nil {
				t.Errorf("StoreMetricById() = %v, want %v", got.value, tt.want.value)
			}
		})
	}
}

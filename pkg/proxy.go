package pkg

import (
	"errors"
	"sync"
	"time"
)

type MetricProxyItem struct {
	value        interface{}
	ttl          int
	creationTime time.Time
}

type MetricProxy struct {
	metrics map[string]*MetricProxyItem
	mutex   sync.RWMutex
}

func NewMetricProxy() *MetricProxy {
	mp := &MetricProxy{}
	mp.metrics = make(map[string]*MetricProxyItem)
	return mp
}

func (mp *MetricProxy) GetMetricById(id string) (*MetricProxyItem, error) {
	mp.mutex.RLock()
	defer mp.mutex.RUnlock()
	if m, ok := mp.metrics[id]; ok {
		if time.Since(m.creationTime).Seconds() > float64(m.ttl) {
			return nil, errors.New("metric ttl has expired")
		}
		return m, nil
	} else {
		return nil, errors.New("metric not found")
	}
}

func (mp *MetricProxy) StoreMetricById(id string, value interface{}, ttl int) {
	mp.mutex.Lock()
	mp.metrics[id] = &MetricProxyItem{
		value:        value,
		creationTime: time.Now(),
		ttl:          ttl,
	}
	mp.mutex.Unlock()
}

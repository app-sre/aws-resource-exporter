package pkg

import (
	"os"
	"strconv"
	"time"
)

func GetEnvIntValue(envname string) (*int, error) {
	if value, ok := os.LookupEnv(envname); ok {
		int64val, err := strconv.ParseInt(value, 10, 0)
		if err != nil {
			return nil, err
		} else {
			intval := int(int64val)
			return &intval, nil
		}
	} else {
		return nil, nil
	}
}

func durationPtr(duration time.Duration) *time.Duration {
	return &duration
}

// Add a new key to the map and return the new map
func WithKeyValue(m map[string]string, key string, value string) map[string]string {
	newMap := make(map[string]string)
	for k, v := range m {
		newMap[k] = v
	}
	newMap[key] = value
	return newMap
}

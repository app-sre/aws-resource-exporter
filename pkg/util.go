package pkg

import (
	"errors"
	"log/slog"
	"net"
	"os"
	"sort"
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

// Determines status from the number of days until EOL
func GetEOLStatus(eol string, thresholds []Threshold) (string, error) {
	eolDate, err := time.Parse("2006-01-02", eol)
	if err != nil {
		return "", err
	}

	if len(thresholds) == 0 {
		return "", errors.New("thresholds slice is empty")
	}

	currentDate := time.Now()
	daysToEOL := int(eolDate.Sub(currentDate).Hours() / 24)

	sort.Slice(thresholds, func(i, j int) bool {
		return thresholds[i].Days < thresholds[j].Days
	})

	for _, threshold := range thresholds {
		if daysToEOL <= threshold.Days {
			return threshold.Name, nil
		}
	}
	return thresholds[len(thresholds)-1].Name, nil
}

// CalculateTotalIPsFromCIDR calculates the total number of IP addresses in a CIDR block using Go's net package
func CalculateTotalIPsFromCIDR(cidrBlock string, logger *slog.Logger) (int64, error) {
	_, ipNet, err := net.ParseCIDR(cidrBlock)
	if err != nil {
		logger.Error("Invalid CIDR format", "cidr", cidrBlock, "err", err)
		return 0, err
	}

	// Get the prefix length
	prefixLength, _ := ipNet.Mask.Size()

	// Validate reasonable prefix length for IPv4 subnets (AWS supports /16 to /28)
	if prefixLength < 16 || prefixLength > 28 {
		logger.Error("Invalid subnet prefix length for AWS", "cidr", cidrBlock, "prefix", prefixLength)
		return 0, errors.New("invalid subnet prefix length for AWS (must be /16 to /28)")
	}

	// For IPv4, calculate 2^(32-prefix_length)
	hostBits := 32 - prefixLength
	totalIPs := int64(1 << hostBits)

	return totalIPs, nil
}

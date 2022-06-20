package main

import (
	"os"
	"strconv"
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

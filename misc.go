package main

import (
	"strconv"
	"time"
)

func boolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func uintToString(v uint) string {
	return strconv.FormatUint(uint64(v), 10)
}

func timeToFloat64(v time.Time) float64 {
	return float64(v.Unix())
}

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

func boolToFloat64(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

func intToString(v int) string {
	return strconv.FormatInt(int64(v), 10)
}

func int64ToString(v int64) string {
	return strconv.FormatInt(v, 10)
}

func uintToString(v uint) string {
	return strconv.FormatUint(uint64(v), 10)
}

func timeToFloat64(v time.Time) float64 {
	return float64(v.Unix())
}

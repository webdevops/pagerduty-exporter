package main

import (
	"strconv"
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

package main

import (
	"strconv"

	"github.com/PagerDuty/go-pagerduty"
)

const (
	PAGERDUTY_MAX_PAGING_LIMIT = 10000
)

func stopPagerdutyPaging(resp pagerduty.APIListObject) bool {
	if !resp.More {
		return true
	}

	if resp.Offset+resp.Limit > PAGERDUTY_MAX_PAGING_LIMIT {
		return true
	}

	return false
}

func boolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func uintToString(v uint) string {
	return strconv.FormatUint(uint64(v), 10)
}

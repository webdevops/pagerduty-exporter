package main

import (
	"testing"
	"time"

	"github.com/PagerDuty/go-pagerduty"
)

func TestDedupKeyFromAlertBody(t *testing.T) {
	tests := map[string]struct {
		body map[string]interface{}
		want string
	}{
		"events v2 alert": {
			body: map[string]interface{}{
				"dedup_key": "d0a86e9b1c2d3e4f5a6b7c8d9e0f1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f",
				"payload":   map[string]interface{}{"summary": "test"},
			},
			want: "d0a86e9b1c2d3e4f5a6b7c8d9e0f1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f",
		},
		"non-events integration": {
			body: map[string]interface{}{"subject": "down"},
			want: "",
		},
		"nil body": {
			body: nil,
			want: "",
		},
		"non-string dedup_key": {
			body: map[string]interface{}{"dedup_key": 42},
			want: "",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			alert := pagerduty.IncidentAlert{Body: tt.body}
			if got := dedupKeyFromAlertBody(alert); got != tt.want {
				t.Errorf("dedupKeyFromAlertBody() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRecordsPrune(t *testing.T) {
	now := time.Now().UTC()
	records := map[string]incidentAlertRecord{
		"old":  {IncidentID: "OLD", CreatedAtMs: now.Add(-40 * 24 * time.Hour).UnixMilli()},
		"kept": {IncidentID: "KEPT", CreatedAtMs: now.Add(-30 * 24 * time.Hour).UnixMilli()},
	}
	cutoff := now.Add(-incidentAlertRetention)
	for key, record := range records {
		if time.UnixMilli(record.CreatedAtMs).Before(cutoff) {
			delete(records, key)
		}
	}
	if _, exists := records["old"]; exists {
		t.Errorf("old record should have been pruned")
	}
	if _, exists := records["kept"]; !exists {
		t.Errorf("kept record should have survived")
	}
}

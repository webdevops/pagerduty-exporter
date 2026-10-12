package main

import (
	"encoding/json"
	"log/slog"
	"time"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/webdevops/go-common/prometheus/collector"
)

// incidentAlertRetention bounds the cached alert records; matches the ~30d
// incident history the Incident collector exposes.
const incidentAlertRetention = 35 * 24 * time.Hour

// incidentAlertLookback is the window re-fetched on a cold start (empty cache);
// older incidents get no alert records until the exporter has run continuously.
const incidentAlertLookback = 2 * time.Hour

type incidentAlertRecord struct {
	IncidentID  string `json:"incidentId"`
	ServiceID   string `json:"serviceId"`
	AlertID     string `json:"alertId"`
	DedupKey    string `json:"dedupKey"`
	Status      string `json:"status"`
	CreatedAtMs int64  `json:"createdAtMs"`
}

type MetricsCollectorIncidentAlert struct {
	collector.Processor

	prometheus struct {
		incidentAlert *prometheus.GaugeVec
	}

	teamListOpt []string
}

func (m *MetricsCollectorIncidentAlert) Setup(collector *collector.Collector) {
	m.Processor.Setup(collector)

	m.prometheus.incidentAlert = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pagerduty_incident_alert_info",
			Help: "PagerDuty incident alert; dedupKey is the Events API v2 dedup_key Alertmanager hashes from its group key",
		},
		[]string{
			"incidentID",
			"serviceID",
			"alertID",
			"dedupKey",
			"status",
		},
	)
	m.Collector.RegisterMetricList("pagerduty_incident_alert_info", m.prometheus.incidentAlert, true)
}

func (m *MetricsCollectorIncidentAlert) Reset() {
}

func (m *MetricsCollectorIncidentAlert) loadRecords() map[string]incidentAlertRecord {
	records := map[string]incidentAlertRecord{}
	if raw := m.Collector.GetData("incidentAlerts"); raw != nil {
		buf, err := json.Marshal(raw)
		if err == nil {
			_ = json.Unmarshal(buf, &records)
		}
	}
	return records
}

func (m *MetricsCollectorIncidentAlert) Collect(callback chan<- func()) {
	records := m.loadRecords()

	listOpts := pagerduty.ListIncidentsOptions{}
	listOpts.Limit = PagerdutyListLimit
	listOpts.Statuses = Opts.PagerDuty.Incident.Statuses
	listOpts.SortBy = "created_at:asc"
	listOpts.Since = time.Now().UTC().Add(-incidentAlertLookback).Format(time.RFC3339)
	// records only exist for incidents already seen, so shrink the window;
	// a short buffer covers scrape jitter and clock skew
	if len(records) > 0 {
		listOpts.Since = time.Now().UTC().Add(-10 * time.Minute).Format(time.RFC3339)
	}

	for {
		list, err := PagerDutyClient.ListIncidentsWithContext(m.Context(), listOpts)
		PrometheusPagerDutyApiCounter.WithLabelValues("ListIncidents").Inc()
		if err != nil {
			panic(err)
		}

		for _, incident := range list.Incidents {
			if _, done := records[incident.ID]; done {
				continue
			}
			alerts, err := m.fetchIncidentAlerts(incident.ID)
			// skip and retry next scrape instead of failing the whole collect
			if err != nil {
				m.Logger().Warn("failed to fetch incident alerts", slog.String("incidentID", incident.ID), slog.String("err", err.Error()))
				continue
			}
			for _, record := range alerts {
				records[record.IncidentID+"/"+record.AlertID] = record
			}
		}

		listOpts.Offset += PagerdutyListLimit
		if stopPagerdutyPaging(list.APIListObject) || listOpts.Offset >= Opts.PagerDuty.Incident.Limit {
			break
		}
	}

	// prune records past retention; the record key carries the incidentID
	// prefix, so this also bounds the seen set
	cutoff := time.Now().UTC().Add(-incidentAlertRetention)
	for key, record := range records {
		if time.UnixMilli(record.CreatedAtMs).Before(cutoff) {
			delete(records, key)
		}
	}

	metricList := m.Collector.GetMetricList("pagerduty_incident_alert_info")
	for _, record := range records {
		createdAt := time.UnixMilli(record.CreatedAtMs)
		metricList.AddTime(prometheus.Labels{
			"incidentID": record.IncidentID,
			"serviceID":  record.ServiceID,
			"alertID":    record.AlertID,
			"dedupKey":   record.DedupKey,
			"status":     record.Status,
		}, createdAt)
	}

	m.Collector.SetData("incidentAlerts", records)
}

func (m *MetricsCollectorIncidentAlert) fetchIncidentAlerts(incidentID string) ([]incidentAlertRecord, error) {
	opts := pagerduty.ListIncidentAlertsOptions{}
	opts.Limit = PagerdutyListLimit

	records := []incidentAlertRecord{}
	for {
		list, err := PagerDutyClient.ListIncidentAlertsWithContext(m.Context(), incidentID, opts)
		PrometheusPagerDutyApiCounter.WithLabelValues("ListIncidentAlerts").Inc()
		if err != nil {
			return nil, err
		}

		for _, alert := range list.Alerts {
			createdAt, _ := time.Parse(time.RFC3339, alert.CreatedAt)
			records = append(records, incidentAlertRecord{
				IncidentID:  incidentID,
				ServiceID:   alert.Service.ID,
				AlertID:     alert.ID,
				DedupKey:    dedupKeyFromAlertBody(alert),
				Status:      alert.Status,
				CreatedAtMs: createdAt.UnixMilli(),
			})
		}

		opts.Offset += PagerdutyListLimit
		if stopPagerdutyPaging(list.APIListObject) || opts.Offset >= PAGERDUTY_MAX_PAGING_LIMIT {
			break
		}
	}
	return records, nil
}

// dedupKeyFromAlertBody reads the Events API v2 dedup_key from the raw alert
// body; non-Events integrations (email, manual) have none
func dedupKeyFromAlertBody(alert pagerduty.IncidentAlert) string {
	if alert.Body == nil {
		return ""
	}
	switch v := alert.Body["dedup_key"].(type) {
	case string:
		return v
	default:
		return ""
	}
}

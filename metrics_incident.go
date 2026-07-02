package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/webdevops/go-common/prometheus/collector"
)

type MetricsCollectorIncident struct {
	collector.Processor

	prometheus struct {
		incident       *prometheus.GaugeVec
		incidentStatus *prometheus.GaugeVec
	}

	teamListOpt  []string
	customFields []string
}

func (m *MetricsCollectorIncident) Setup(collector *collector.Collector) {
	m.Processor.Setup(collector)

	m.customFields = Opts.PagerDuty.Incident.CustomFields

	incidentLabels := []string{
		"incidentID",
		"serviceID",
		"incidentUrl",
		"incidentNumber",
		"title",
		"status",
		"urgency",
		"acknowledged",
		"assigned",
		"type",
		"time",
		"resolveReason",
		"resolveReasonIncidentID",
	}
	for _, customField := range m.customFields {
		incidentLabels = append(incidentLabels, incidentCustomFieldLabel(customField))
	}

	m.prometheus.incident = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pagerduty_incident_info",
			Help: "PagerDuty incident",
		},
		incidentLabels,
	)
	m.Collector.RegisterMetricList("pagerduty_incident_info", m.prometheus.incident, true)

	m.prometheus.incidentStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pagerduty_incident_status",
			Help: "PagerDuty incident status",
		},
		[]string{
			"incidentID",
			"userID",
			"time",
			"type",
		},
	)
	m.Collector.RegisterMetricList("pagerduty_incident_status", m.prometheus.incidentStatus, true)
}

func (m *MetricsCollectorIncident) Reset() {
}

func (m *MetricsCollectorIncident) Collect(callback chan<- func()) {
	listOpts := pagerduty.ListIncidentsOptions{}
	listOpts.Limit = PagerdutyListLimit
	listOpts.Statuses = Opts.PagerDuty.Incident.Statuses
	listOpts.Offset = 0
	listOpts.SortBy = "created_at:desc"

	if len(m.teamListOpt) > 0 {
		listOpts.TeamIDs = m.teamListOpt
	}

	incidentMetricList := m.Collector.GetMetricList("pagerduty_incident_info")
	incidentStatusMetricList := m.Collector.GetMetricList("pagerduty_incident_status")

	for {
		m.Logger().Debug("fetch incidents", slog.Uint64("offset", uint64(listOpts.Offset)), slog.Uint64("limit", uint64(listOpts.Limit)))

		list, err := PagerDutyClient.ListIncidentsWithContext(m.Context(), listOpts)
		PrometheusPagerDutyApiCounter.WithLabelValues("ListIncidents").Inc()

		if err != nil {
			panic(err)
		}

		for _, incident := range list.Incidents {
			// info
			createdAt, _ := time.Parse(time.RFC3339, incident.CreatedAt)

			incidentLabels := prometheus.Labels{
				"incidentID":              incident.ID,
				"serviceID":               incident.Service.ID,
				"incidentUrl":             incident.HTMLURL,
				"incidentNumber":          uintToString(incident.IncidentNumber),
				"title":                   incident.Title,
				"status":                  incident.Status,
				"urgency":                 incident.Urgency,
				"acknowledged":            boolToString(len(incident.Acknowledgements) >= 1),
				"assigned":                boolToString(len(incident.Assignments) >= 1),
				"type":                    incident.Type,
				"time":                    createdAt.Format(Opts.PagerDuty.Incident.TimeFormat),
				"resolveReason":           incident.ResolveReason.Type,
				"resolveReasonIncidentID": incident.ResolveReason.Incident.ID,
			}

			// custom field values are not available via include[] on the list
			// endpoint, so fetch them per incident when configured
			if len(m.customFields) > 0 {
				customFieldValues, err := m.getIncidentCustomFieldValues(m.Context(), incident.ID)
				if err != nil {
					m.Logger().Warn("failed to fetch incident custom field values",
						slog.String("incidentID", incident.ID),
						slog.Any("error", err),
					)
					customFieldValues = map[string]string{}
				}
				for _, customField := range m.customFields {
					incidentLabels[incidentCustomFieldLabel(customField)] = customFieldValues[customField]
				}
			}

			incidentMetricList.AddTime(incidentLabels, createdAt)

			// acknowledgement
			for _, acknowledgement := range incident.Acknowledgements {
				createdAt, _ := time.Parse(time.RFC3339, acknowledgement.At)
				incidentStatusMetricList.AddTime(prometheus.Labels{
					"incidentID": incident.ID,
					"userID":     acknowledgement.Acknowledger.ID,
					"time":       createdAt.Format(Opts.PagerDuty.Incident.TimeFormat),
					"type":       "acknowledgement",
				}, createdAt)
			}

			// assignment
			for _, assignment := range incident.Assignments {
				createdAt, _ := time.Parse(time.RFC3339, assignment.At)
				incidentStatusMetricList.AddTime(prometheus.Labels{
					"incidentID": incident.ID,
					"userID":     assignment.Assignee.ID,
					"time":       createdAt.Format(Opts.PagerDuty.Incident.TimeFormat),
					"type":       "assignment",
				}, createdAt)
			}

			// lastChange
			changedAt, _ := time.Parse(time.RFC3339, incident.LastStatusChangeAt)
			incidentStatusMetricList.AddTime(prometheus.Labels{
				"incidentID": incident.ID,
				"userID":     incident.LastStatusChangeBy.ID,
				"time":       changedAt.Format(Opts.PagerDuty.Incident.TimeFormat),
				"type":       "lastChange",
			}, changedAt)
		}

		listOpts.Offset += PagerdutyListLimit
		if stopPagerdutyPaging(list.APIListObject) || listOpts.Offset >= Opts.PagerDuty.Incident.Limit {
			break
		}
	}
}

// getIncidentCustomFieldValues fetches the custom field values for a single
// incident via GET /incidents/{id}/custom_fields/values. These values are not
// exposed through include[] on the incident list endpoint, so a dedicated
// request per incident is required.
func (m *MetricsCollectorIncident) getIncidentCustomFieldValues(ctx context.Context, incidentID string) (map[string]string, error) {
	requestURL := fmt.Sprintf(
		"%s/incidents/%s/custom_fields/values",
		strings.TrimRight(Opts.PagerDuty.ApiEndpoint, "/"),
		url.PathEscape(incidentID),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := PagerDutyClient.Do(req, true)
	PrometheusPagerDutyApiCounter.WithLabelValues("GetIncidentCustomFieldValues").Inc()
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected HTTP status %q while fetching custom field values for incident %s", resp.Status, incidentID)
	}

	var parsed struct {
		CustomFields []struct {
			Name  string      `json:"name"`
			Value interface{} `json:"value"`
		} `json:"custom_fields"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}

	values := make(map[string]string, len(parsed.CustomFields))
	for _, customField := range parsed.CustomFields {
		values[customField.Name] = customFieldValueToString(customField.Value)
	}

	return values, nil
}

// incidentCustomFieldLabel converts a PagerDuty custom field name into a valid
// Prometheus label name, prefixed to avoid clashing with built-in labels.
func incidentCustomFieldLabel(name string) string {
	var sb strings.Builder
	sb.WriteString("customfield_")
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_':
			sb.WriteRune(r)
		default:
			sb.WriteRune('_')
		}
	}
	return sb.String()
}

// customFieldValueToString renders a custom field value (string, bool, number
// or list of those) into a label string value.
func customFieldValueToString(value interface{}) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case bool:
		return strconv.FormatBool(v)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case []interface{}:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			parts = append(parts, customFieldValueToString(item))
		}
		return strings.Join(parts, ",")
	default:
		return fmt.Sprintf("%v", v)
	}
}

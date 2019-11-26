package main

import (
	"context"
	"github.com/PagerDuty/go-pagerduty"
	"github.com/prometheus/client_golang/prometheus"
	"time"
)

type MetricsCollectorIncident struct {
	CollectorProcessorGeneral

	prometheus struct {
		incident       *prometheus.GaugeVec
		incidentStatus *prometheus.GaugeVec
	}

	teamListOpt []string
}

func (m *MetricsCollectorIncident) Setup(collector *CollectorGeneral) {
	m.CollectorReference = collector

	m.prometheus.incident = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pagerduty_incident_info",
			Help: "PagerDuty incident",
		},
		[]string{
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
		},
	)

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

	prometheus.MustRegister(m.prometheus.incident)
	prometheus.MustRegister(m.prometheus.incidentStatus)
}

func (m *MetricsCollectorIncident) Reset() {
	m.prometheus.incident.Reset()
	m.prometheus.incidentStatus.Reset()
}

func (m *MetricsCollectorIncident) Collect(ctx context.Context, callback chan<- func()) {
	listOpts := pagerduty.ListIncidentsOptions{}
	listOpts.Limit = PAGERDUTY_LIST_LIMIT
	listOpts.Statuses = []string{"triggered", "acknowledged"}
	listOpts.Offset = 0

	if len(m.teamListOpt) > 0 {
		listOpts.TeamIDs = m.teamListOpt
	}

	incidentMetricList := MetricCollectorList{}
	incidentStatusMetricList := MetricCollectorList{}

	for {
		Logger.Verbosef(" - fetch incidents (offset: %v, limit:%v)", listOpts.Offset, listOpts.Limit)

		list, err := PagerDutyClient.ListIncidents(listOpts)
		m.CollectorReference.PrometheusApiCounter().WithLabelValues("ListIncidents").Inc()

		if err != nil {
			panic(err)
		}

		for _, incident := range list.Incidents {
			// info
			createdAt, _ := time.Parse(time.RFC3339, incident.CreatedAt)

			incidentMetricList.AddTime(prometheus.Labels{
				"incidentID":     incident.ID,
				"serviceID":      incident.Service.ID,
				"incidentUrl":    incident.HTMLURL,
				"incidentNumber": uintToString(incident.IncidentNumber),
				"title":          incident.Title,
				"status":         incident.Status,
				"urgency":        incident.Urgency,
				"acknowledged":   boolToString(len(incident.Acknowledgements) >= 1),
				"assigned":       boolToString(len(incident.Assignments) >= 1),
				"type":           incident.Type,
				"time":           createdAt.Format(opts.PagerDutyIncidentTimeFormat),
			}, createdAt)

			// acknowledgement
			for _, acknowledgement := range incident.Acknowledgements {
				createdAt, _ := time.Parse(time.RFC3339, acknowledgement.At)
				incidentStatusMetricList.AddTime(prometheus.Labels{
					"incidentID": incident.ID,
					"userID":     acknowledgement.Acknowledger.ID,
					"time":       createdAt.Format(opts.PagerDutyIncidentTimeFormat),
					"type":       "acknowledgement",
				}, createdAt)
			}

			// assignment
			for _, assignment := range incident.Assignments {
				createdAt, _ := time.Parse(time.RFC3339, assignment.At)
				incidentStatusMetricList.AddTime(prometheus.Labels{
					"incidentID": incident.ID,
					"userID":     assignment.Assignee.ID,
					"time":       createdAt.Format(opts.PagerDutyIncidentTimeFormat),
					"type":       "assignment",
				}, createdAt)
			}

			// lastChange
			changedAt, _ := time.Parse(time.RFC3339, incident.LastStatusChangeAt)
			incidentStatusMetricList.AddTime(prometheus.Labels{
				"incidentID": incident.ID,
				"userID":     incident.LastStatusChangeBy.ID,
				"time":       changedAt.Format(opts.PagerDutyIncidentTimeFormat),
				"type":       "lastChange",
			}, changedAt)
		}

		listOpts.Offset += list.Limit
		if !list.More {
			break
		}
	}

	// set metrics
	callback <- func() {
		incidentMetricList.GaugeSet(m.prometheus.incident)
		incidentStatusMetricList.GaugeSet(m.prometheus.incidentStatus)
	}
}

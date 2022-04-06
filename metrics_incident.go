package main

import (
	"context"
	"time"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/prometheus/client_golang/prometheus"
	prometheusCommon "github.com/webdevops/go-common/prometheus"
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
	listOpts.Limit = PagerdutyListLimit
	listOpts.Statuses = opts.PagerDuty.Incident.Statuses
	listOpts.Offset = 0
	listOpts.SortBy = "created_at:desc"

	if len(m.teamListOpt) > 0 {
		listOpts.TeamIDs = m.teamListOpt
	}

	incidentMetricList := prometheusCommon.NewMetricsList()
	incidentStatusMetricList := prometheusCommon.NewMetricsList()

	for {
		m.logger().Debugf("fetch incidents (offset: %v, limit:%v)", listOpts.Offset, listOpts.Limit)

		list, err := PagerDutyClient.ListIncidents(listOpts)
		m.CollectorReference.PrometheusAPICounter().WithLabelValues("ListIncidents").Inc()

		if err != nil {
			m.logger().Panic(err)
		}

		for _, incident := range list.Incidents {
			// workaround for https://github.com/PagerDuty/go-pagerduty/issues/218
			incidentId := incident.ID
			if incidentId == "" && incident.Id != "" {
				incidentId = incident.Id
			}

			// info
			createdAt, _ := time.Parse(time.RFC3339, incident.CreatedAt)

			incidentMetricList.AddTime(prometheus.Labels{
				"incidentID":     incidentId,
				"serviceID":      incident.Service.ID,
				"incidentUrl":    incident.HTMLURL,
				"incidentNumber": uintToString(incident.IncidentNumber),
				"title":          incident.Title,
				"status":         incident.Status,
				"urgency":        incident.Urgency,
				"acknowledged":   boolToString(len(incident.Acknowledgements) >= 1),
				"assigned":       boolToString(len(incident.Assignments) >= 1),
				"type":           incident.Type,
				"time":           createdAt.Format(opts.PagerDuty.Incident.TimeFormat),
			}, createdAt)

			// acknowledgement
			for _, acknowledgement := range incident.Acknowledgements {
				createdAt, _ := time.Parse(time.RFC3339, acknowledgement.At)
				incidentStatusMetricList.AddTime(prometheus.Labels{
					"incidentID": incidentId,
					"userID":     acknowledgement.Acknowledger.ID,
					"time":       createdAt.Format(opts.PagerDuty.Incident.TimeFormat),
					"type":       "acknowledgement",
				}, createdAt)
			}

			// assignment
			for _, assignment := range incident.Assignments {
				createdAt, _ := time.Parse(time.RFC3339, assignment.At)
				incidentStatusMetricList.AddTime(prometheus.Labels{
					"incidentID": incidentId,
					"userID":     assignment.Assignee.ID,
					"time":       createdAt.Format(opts.PagerDuty.Incident.TimeFormat),
					"type":       "assignment",
				}, createdAt)
			}

			// lastChange
			changedAt, _ := time.Parse(time.RFC3339, incident.LastStatusChangeAt)
			incidentStatusMetricList.AddTime(prometheus.Labels{
				"incidentID": incidentId,
				"userID":     incident.LastStatusChangeBy.ID,
				"time":       changedAt.Format(opts.PagerDuty.Incident.TimeFormat),
				"type":       "lastChange",
			}, changedAt)
		}

		listOpts.Offset += PagerdutyListLimit
		if !list.More || listOpts.Offset >= opts.PagerDuty.Incident.Limit {
			break
		}
	}

	// set metrics
	callback <- func() {
		incidentMetricList.GaugeSet(m.prometheus.incident)
		incidentStatusMetricList.GaugeSet(m.prometheus.incidentStatus)
	}
}

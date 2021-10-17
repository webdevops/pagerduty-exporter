package main

import (
	"context"
	"github.com/PagerDuty/go-pagerduty"
	"github.com/prometheus/client_golang/prometheus"
	prometheusCommon "github.com/webdevops/go-prometheus-common"
	"strings"
	"time"
)

type MetricsCollectorSummary struct {
	CollectorProcessorGeneral

	prometheus struct {
		incidentCount             *prometheus.GaugeVec
		incidentResolveDuration   *prometheus.HistogramVec
		incidentStatusChangeCount *prometheus.CounterVec
	}

	teamListOpt []string
}

func (m *MetricsCollectorSummary) Setup(collector *CollectorGeneral) {
	m.CollectorReference = collector

	m.prometheus.incidentCount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pagerduty_summary_incident_count",
			Help: "PagerDuty overall incident count for summary duration",
		},
		[]string{
			"serviceID",
			"status",
			"urgency",
			"priority",
		},
	)
	prometheus.MustRegister(m.prometheus.incidentCount)

	m.prometheus.incidentResolveDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "pagerduty_summary_incident_resolve_duration",
			Help: "PagerDuty overall incident resolve duration for summary duration",
			Buckets: []float64{
				5 * 60,            // 5 min
				15 * 60,           // 15 min
				30 * 60,           // 30 min
				1 * 60 * 60,       // 1 hour
				3 * 60 * 60,       // 3 hours
				6 * 60 * 60,       // 6 hours
				12 * 60 * 60,      // 12 hours
				1 * 24 * 60 * 60,  // 1 day
				5 * 24 * 60 * 60,  // 5 days (workday)
				7 * 24 * 60 * 60,  // 7 days (week)
				14 * 24 * 60 * 60, // 2 weeks
				31 * 24 * 60 * 60, // 1 month
			},
		},
		[]string{
			"serviceID",
			"urgency",
			"priority",
		},
	)
	prometheus.MustRegister(m.prometheus.incidentResolveDuration)

	m.prometheus.incidentStatusChangeCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pagerduty_summary_incident_statuschange_count",
			Help: "PagerDuty number of observed status changes for incidents",
		},
		[]string{
			"serviceID",
			"status",
			"urgency",
			"priority",
		},
	)
	prometheus.MustRegister(m.prometheus.incidentStatusChangeCount)
}

func (m *MetricsCollectorSummary) Reset() {
	m.prometheus.incidentCount.Reset()
	m.prometheus.incidentResolveDuration.Reset()
}

func (m *MetricsCollectorSummary) Collect(ctx context.Context, callback chan<- func()) {
	m.collectIncidents(ctx, callback)
}

func (m *MetricsCollectorSummary) collectIncidents(ctx context.Context, callback chan<- func()) {
	now := time.Now().UTC()

	listOpts := pagerduty.ListIncidentsOptions{}
	listOpts.Limit = PagerdutyListLimit
	listOpts.Since = now.Add(-opts.PagerDuty.Summary.Since).Format(time.RFC3339)
	listOpts.Until = now.Format(time.RFC3339)
	listOpts.Offset = 0

	if len(m.teamListOpt) > 0 {
		listOpts.TeamIDs = m.teamListOpt
	}

	overallIncidentCountMetricList := prometheusCommon.NewHashedMetricsList()
	overallIncidentResolveDurationMetricList := prometheusCommon.NewMetricsList()
	changedIncidentCountMetricList := prometheusCommon.NewHashedMetricsList()

	for {
		m.logger().Debugf("fetch incidents (offset: %v, limit:%v)", listOpts.Offset, listOpts.Limit)

		list, err := PagerDutyClient.ListIncidents(listOpts)
		m.CollectorReference.PrometheusAPICounter().WithLabelValues("ListIncidents").Inc()

		if err != nil {
			m.logger().Panic(err)
		}

		for _, incident := range list.Incidents {
			createdAt, _ := time.Parse(time.RFC3339, incident.CreatedAt)
			lastStatusChangeAt, _ := time.Parse(time.RFC3339, incident.LastStatusChangeAt)

			incidentPriority := ""
			if incident.Priority != nil {
				incidentPriority = incident.Priority.Name
			}

			overallIncidentCountMetricList.Inc(prometheus.Labels{
				"serviceID": incident.Service.ID,
				"status":    incident.Status,
				"urgency":   incident.Urgency,
				"priority":  incidentPriority,
			})

			switch strings.ToLower(incident.Status) {
			case "resolved":
				// info
				resolveDuration := lastStatusChangeAt.Sub(createdAt)

				overallIncidentResolveDurationMetricList.AddDuration(prometheus.Labels{
					"serviceID": incident.Service.ID,
					"urgency":   incident.Urgency,
					"priority":  incidentPriority,
				}, resolveDuration)
			}

			if m.CollectorReference.collectionLastTime != nil {
				if createdAt.After(*m.CollectorReference.collectionLastTime) {
					changedIncidentCountMetricList.Inc(prometheus.Labels{
						"serviceID": incident.Service.ID,
						"status":    "created",
						"urgency":   incident.Urgency,
						"priority":  incidentPriority,
					})
				} else if lastStatusChangeAt.After(*m.CollectorReference.collectionLastTime) {
					changedIncidentCountMetricList.Inc(prometheus.Labels{
						"serviceID": incident.Service.ID,
						"status":    incident.Status,
						"urgency":   incident.Urgency,
						"priority":  incidentPriority,
					})
				}
			}
		}

		listOpts.Offset += list.Limit
		if !list.More {
			break
		}
	}

	// set metrics
	callback <- func() {
		overallIncidentCountMetricList.GaugeSet(m.prometheus.incidentCount)
		overallIncidentResolveDurationMetricList.HistogramSet(m.prometheus.incidentResolveDuration)
		changedIncidentCountMetricList.CounterAdd(m.prometheus.incidentStatusChangeCount)
	}
}
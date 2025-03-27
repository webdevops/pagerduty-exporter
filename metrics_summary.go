package main

import (
	"strings"
	"time"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/prometheus/client_golang/prometheus"
	prometheusCommon "github.com/webdevops/go-common/prometheus"
	"github.com/webdevops/go-common/prometheus/collector"
)

type MetricsCollectorSummary struct {
	collector.Processor

	prometheus struct {
		incidentCount               *prometheus.GaugeVec
		incidentResolveDuration     *prometheus.HistogramVec
		incidentAcknowledgeDuration *prometheus.HistogramVec
		incidentStatusChangeCount   *prometheus.CounterVec
	}

	teamListOpt []string
}

func (m *MetricsCollectorSummary) Setup(collector *collector.Collector) {
	m.Processor.Setup(collector)

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

	m.prometheus.incidentAcknowledgeDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "pagerduty_summary_incident_acknowledge_duration",
			Help: "PagerDuty overall incident acknowledge duration for summary duration",
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
	prometheus.MustRegister(m.prometheus.incidentAcknowledgeDuration)

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
	m.prometheus.incidentAcknowledgeDuration.Reset()
}

func (m *MetricsCollectorSummary) Collect(callback chan<- func()) {
	m.collectIncidents(callback)
}

func (m *MetricsCollectorSummary) collectIncidents(callback chan<- func()) {
	now := time.Now().UTC()

	listOpts := pagerduty.ListIncidentsOptions{
		Includes: []string{"acknowledgers"},
	}
	listOpts.Limit = PagerdutyListLimit
	listOpts.Since = now.Add(-Opts.PagerDuty.Summary.Since).Format(time.RFC3339)
	listOpts.Until = now.Format(time.RFC3339)
	listOpts.Offset = 0
	listOpts.Statuses = []string{"triggered", "acknowledged", "resolved"}

	if len(m.teamListOpt) > 0 {
		listOpts.TeamIDs = m.teamListOpt
	}

	overallIncidentCountMetricList := prometheusCommon.NewHashedMetricsList()
	overallIncidentResolveDurationMetricList := prometheusCommon.NewMetricsList()
	overallIncidentAcknowledgeDurationMetricList := prometheusCommon.NewMetricsList()
	changedIncidentCountMetricList := prometheusCommon.NewHashedMetricsList()

	for {
		m.Logger().Debugf("fetch incidents (offset: %v, limit:%v, since:%v, until:%v)", listOpts.Offset, listOpts.Limit, listOpts.Since, listOpts.Until)

		list, err := PagerDutyClient.ListIncidentsWithContext(m.Context(), listOpts)
		PrometheusPagerDutyApiCounter.WithLabelValues("ListIncidents").Inc()

		if err != nil {
			m.Logger().Panic(err)
		}

		for _, incident := range list.Incidents {
			createdAt, _ := time.Parse(time.RFC3339, incident.CreatedAt)
			resolvedAt, _ := time.Parse(time.RFC3339, incident.ResolvedAt)
			acknowledgedAt := time.Now()

			incidentLogEntries, err := PagerDutyClient.ListIncidentLogEntriesWithContext(m.Context(), incident.ID, pagerduty.ListIncidentLogEntriesOptions{
				Limit:      PagerdutyListLimit,
				IsOverview: true,
			})
			if err != nil {
				m.Logger().Panic(err)
			}

			for _, entry := range incidentLogEntries.LogEntries {
				at, _ := time.Parse(time.RFC3339, entry.CreatedAt)
				if strings.HasPrefix(entry.Type, "acknowledge_log_entry") && acknowledgedAt.After(at) {
					acknowledgedAt = at
				}
			}

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
				resolveDuration := resolvedAt.Sub(createdAt)

				overallIncidentResolveDurationMetricList.AddDuration(prometheus.Labels{
					"serviceID": incident.Service.ID,
					"urgency":   incident.Urgency,
					"priority":  incidentPriority,
				}, resolveDuration)
				fallthrough
			case "acknowledged":
				// info
				acknowledgeDuration := acknowledgedAt.Sub(createdAt)

				overallIncidentAcknowledgeDurationMetricList.AddDuration(prometheus.Labels{
					"serviceID": incident.Service.ID,
					"urgency":   incident.Urgency,
					"priority":  incidentPriority,
				}, acknowledgeDuration)
			}

			if m.GetLastScapeTime() != nil {
				if createdAt.After(*m.GetLastScapeTime()) {
					changedIncidentCountMetricList.Inc(prometheus.Labels{
						"serviceID": incident.Service.ID,
						"status":    "created",
						"urgency":   incident.Urgency,
						"priority":  incidentPriority,
					})
				} else if acknowledgedAt.After(*m.GetLastScapeTime()) || resolvedAt.After(*m.GetLastScapeTime()) {
					changedIncidentCountMetricList.Inc(prometheus.Labels{
						"serviceID": incident.Service.ID,
						"status":    incident.Status,
						"urgency":   incident.Urgency,
						"priority":  incidentPriority,
					})
				}
			}
		}

		listOpts.Offset += PagerdutyListLimit
		if stopPagerdutyPaging(list.APIListObject) {
			break
		}
	}

	// set metrics
	callback <- func() {
		overallIncidentCountMetricList.GaugeSet(m.prometheus.incidentCount)
		overallIncidentResolveDurationMetricList.HistogramSet(m.prometheus.incidentResolveDuration)
		overallIncidentAcknowledgeDurationMetricList.HistogramSet(m.prometheus.incidentAcknowledgeDuration)
		changedIncidentCountMetricList.CounterAdd(m.prometheus.incidentStatusChangeCount)
	}
}

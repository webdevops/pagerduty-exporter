package main

import (
	"strconv"
	"time"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/prometheus/client_golang/prometheus"
	prometheusCommon "github.com/webdevops/go-common/prometheus"
	"github.com/webdevops/go-common/prometheus/collector"
)

type MetricsCollectorAnalytics struct {
	collector.Processor

	prometheus struct {
		analytics       *prometheus.GaugeVec
		analyticsStatus *prometheus.GaugeVec
	}

	teamListOpt []string
}

func (m *MetricsCollectorAnalytics) Setup(collector *collector.Collector) {
	m.Processor.Setup(collector)

	m.prometheus.analytics = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pagerduty_analytics_incident",
			Help: "PagerDuty analytics",
		},
		[]string{
			"serviceID",
			"serviceName",
			"teamID",
			"teamName",
			"meanSecondsToResolve",
			"meanSecondsToFirstAck",
			"meanSecondsToEngage",
			"meanSecondsToMobilize",
			"meanEngagedSeconds",
			"meanEngagedUserCount",
			"totalEscalationCount",
			"meanAssignmentCount",
			"totalBusinessHourInterruptionspty",
			"totalSleepHourInterruptions",
			"totalOffHourInterruptions",
			"totalSnoozedSeconds",
			"totalEngagedSeconds",
			"totalIncidentCount",
			"upTimePct",
			"userDefinedEffortSeconds",
			"rangeStart",
		},
	)

	prometheus.MustRegister(m.prometheus.analytics)
}

func (m *MetricsCollectorAnalytics) Reset() {
	m.prometheus.analytics.Reset()
}

func (m *MetricsCollectorAnalytics) Collect(callback chan<- func()) {
	analyticsRequest := pagerduty.AnalyticsRequest{}
	analyticsFilter := pagerduty.AnalyticsFilter{}
	now := time.Now()

	analyticsFilter.CreatedAtStart = now.Add(time.Duration(-24*7) * time.Hour).Format(time.RFC3339)
	analyticsFilter.CreatedAtEnd = now.Format(time.RFC3339)
	analyticsFilter.Urgency = "high"

	analyticsRequest.Filters = &analyticsFilter
	analyticsRequest.AggregateUnit = "day"
	analyticsRequest.TimeZone = "Etc/UTC"

	analyticsIncidentMetric := prometheusCommon.NewMetricsList()

	for {
		m.Logger().Debugf("fetch analytics")

		a, err := PagerDutyClient.GetAggregatedIncidentData(m.Context(), analyticsRequest)
		PrometheusPagerDutyApiCounter.WithLabelValues("AggregatedIncident").Inc()

		if err != nil {
			m.Logger().Panic(err)
		}

		for _, analytic := range a.Data {
			// info
			createdAt, _ := time.Parse(time.RFC3339, analytics.Filter)

			analyticsIncidentMetric.AddTime(prometheus.Labels{
				"serviceID":                      analytic.ServiceID,
				"serviceName":                    analytic.ServiceName,
				"teamID":                         analytic.TeamID,
				"teamName":                       analytic.TeamName,
				"meanSecondsToResolve":           strconv.Itoa(analytic.MeanSecondsToResolve),
				"meanSecondsToFirstAck":          strconv.Itoa(analytic.MeanSecondsToFirstAck),
				"meanSecondsToEngage":            strconv.Itoa(analytic.MeanSecondsToEngage),
				"meanSecondsToMobilize":          strconv.Itoa(analytic.MeanSecondsToMobilize),
				"meanEngagedSeconds":             strconv.Itoa(analytic.MeanEngagedSeconds),
				"meanEngagedUserCount":           strconv.Itoa(analytic.MeanEngagedUserCount),
				"totalEscalationCount":           strconv.Itoa(analytic.TotalEscalationCount),
				"meanAssignmentCount":            strconv.Itoa(analytic.MeanAssignmentCount),
				"totalBusinessHourInterruptions": strconv.Itoa(analytic.TotalBusinessHourInterruptions),
				"totalSleepHourInterruptions":    strconv.Itoa(analytic.TotalSleepHourInterruptions),
				"totalOffHourInterruptions":      strconv.Itoa(analytic.TotalOffHourInterruptions),
				"totalSnoozedSeconds":            strconv.Itoa(analytic.TotalSnoozedSeconds),
				"totalEngagedSeconds":            strconv.Itoa(analytic.TotalEngagedSeconds),
				"totalIncidentCount":             strconv.Itoa(analytic.TotalIncidentCount),
				"upTimePct":                      strconv.FormatFloat(analytic.UpTimePct),
				"userDefinedEffortSeconds":       strconv.Itoa(analytic.UserDefinedEffortSeconds),
				"rangeStart":                     analytic.RangeStart,
			}, createdAt)

			// acknowledgement
			for _, acknowledgement := range analytics.Acknowledgements {
				createdAt, _ := time.Parse(time.RFC3339, acknowledgement.At)
				analyticsStatusMetricList.AddTime(prometheus.Labels{
					"analyticsID": analytics.ID,
					"userID":      acknowledgement.Acknowledger.ID,
					"time":        createdAt.Format(opts.PagerDuty.Incident.TimeFormat),
					"type":        "acknowledgement",
				}, createdAt)
			}

			// assignment
			for _, assignment := range analytics.Assignments {
				createdAt, _ := time.Parse(time.RFC3339, assignment.At)
				analyticsStatusMetricList.AddTime(prometheus.Labels{
					"analyticsID": analytics.ID,
					"userID":      assignment.Assignee.ID,
					"time":        createdAt.Format(opts.PagerDuty.Incident.TimeFormat),
					"type":        "assignment",
				}, createdAt)
			}

			// lastChange
			changedAt, _ := time.Parse(time.RFC3339, analytics.LastStatusChangeAt)
			analyticsStatusMetricList.AddTime(prometheus.Labels{
				"analyticsID": analytics.ID,
				"userID":      analytics.LastStatusChangeBy.ID,
				"time":        changedAt.Format(opts.PagerDuty.Incident.TimeFormat),
				"type":        "lastChange",
			}, changedAt)
		}

		listOpts.Offset += PagerdutyListLimit
		if !list.More || listOpts.Offset >= opts.PagerDuty.Incident.Limit {
			break
		}
	}

	// set metrics
	callback <- func() {
		analyticsMetricList.GaugeSet(m.prometheus.analytics)
		analyticsStatusMetricList.GaugeSet(m.prometheus.analyticsStatus)
	}
}

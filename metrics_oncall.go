package main

import (
	"log/slog"
	"time"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/webdevops/go-common/prometheus/collector"
)

type MetricsCollectorOncall struct {
	collector.Processor

	prometheus struct {
		scheduleOnCall *prometheus.GaugeVec
	}
}

func (m *MetricsCollectorOncall) Setup(collector *collector.Collector) {
	m.Processor.Setup(collector)

	m.prometheus.scheduleOnCall = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pagerduty_schedule_oncall",
			Help: "PagerDuty schedule oncall",
		},
		[]string{"scheduleID", "userID", "escalationLevel", "type"},
	)
	m.Collector.RegisterMetricList("pagerduty_schedule_oncall", m.prometheus.scheduleOnCall, true)
}

func (m *MetricsCollectorOncall) Reset() {
}

func (m *MetricsCollectorOncall) Collect(callback chan<- func()) {
	listOpts := pagerduty.ListOnCallOptions{}
	listOpts.Limit = PagerdutyListLimit
	listOpts.Earliest = true
	listOpts.Offset = 0

	onCallMetricList := m.Collector.GetMetricList("pagerduty_schedule_oncall")

	for {
		m.Logger().Debug("fetch schedule oncalls", slog.Uint64("offset", uint64(listOpts.Offset)), slog.Uint64("limit", uint64(listOpts.Limit)))

		list, err := PagerDutyClient.ListOnCallsWithContext(m.Context(), listOpts)
		PrometheusPagerDutyApiCounter.WithLabelValues("ListOnCalls").Inc()

		if err != nil {
			panic(err)
		}

		for _, oncall := range list.OnCalls {
			startTime, _ := time.Parse(time.RFC3339, oncall.Start)
			endTime, _ := time.Parse(time.RFC3339, oncall.End)

			startValue := float64(startTime.Unix())
			endValue := float64(endTime.Unix())

			if startValue < 0 {
				startValue = 1
			}

			if endValue < 0 {
				endValue = 1
			}

			// start
			onCallMetricList.Add(prometheus.Labels{
				"scheduleID":      oncall.Schedule.ID,
				"userID":          oncall.User.ID,
				"escalationLevel": uintToString(oncall.EscalationLevel),
				"type":            "startTime",
			}, startValue)

			// end
			onCallMetricList.Add(prometheus.Labels{
				"scheduleID":      oncall.Schedule.ID,
				"userID":          oncall.User.ID,
				"escalationLevel": uintToString(oncall.EscalationLevel),
				"type":            "endTime",
			}, endValue)
		}

		// loop
		listOpts.Offset += list.Limit
		if stopPagerdutyPaging(list.APIListObject) {
			break
		}
	}
}

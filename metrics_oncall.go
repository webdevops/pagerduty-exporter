package main

import (
	"context"
	"github.com/PagerDuty/go-pagerduty"
	"github.com/prometheus/client_golang/prometheus"
	"time"
)

type MetricsCollectorOncall struct {
	CollectorProcessorGeneral

	prometheus struct {
		scheduleOnCall *prometheus.GaugeVec
	}
}

func (m *MetricsCollectorOncall) Setup(collector *CollectorGeneral) {
	m.CollectorReference = collector

	m.prometheus.scheduleOnCall = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pagerduty_schedule_oncall",
			Help: "PagerDuty schedule oncall",
		},
		[]string{"scheduleID", "userID", "escalationLevel", "type"},
	)

	prometheus.MustRegister(m.prometheus.scheduleOnCall)
}

func (m *MetricsCollectorOncall) Reset() {
	m.prometheus.scheduleOnCall.Reset()
}

func (m *MetricsCollectorOncall) Collect(ctx context.Context, callback chan<- func()) {
	listOpts := pagerduty.ListOnCallOptions{}
	listOpts.Limit = PAGERDUTY_LIST_LIMIT
	listOpts.Earliest = true
	listOpts.Offset = 0

	onCallMetricList := MetricCollectorList{}

	for {
		Logger.Verbosef(" - fetch schedule oncalls (offset: %v, limit:%v)", listOpts.Offset, listOpts.Limit)

		list, err := PagerDutyClient.ListOnCalls(listOpts)
		m.CollectorReference.PrometheusApiCounter().WithLabelValues("ListOnCalls").Inc()

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
		if !list.More {
			break
		}
	}

	// set metrics
	callback <- func() {
		onCallMetricList.GaugeSet(m.prometheus.scheduleOnCall)
	}
}

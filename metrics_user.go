package main

import (
	"context"
	"github.com/PagerDuty/go-pagerduty"
	"github.com/prometheus/client_golang/prometheus"
)

type MetricsCollectorUser struct {
	CollectorProcessorGeneral

	prometheus struct {
		user *prometheus.GaugeVec
	}

	teamListOpt []string
}

func (m *MetricsCollectorUser) Setup(collector *CollectorGeneral) {
	m.CollectorReference = collector

	m.prometheus.user = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pagerduty_user_info",
			Help: "PagerDuty user",
		},
		[]string{
			"userID",
			"userName",
			"userMail",
			"userAvatar",
			"userColor",
			"userJobTitle",
			"userRole",
			"userTimezone",
		},
	)

	prometheus.MustRegister(m.prometheus.user)
}

func (m *MetricsCollectorUser) Reset() {
	m.prometheus.user.Reset()
}

func (m *MetricsCollectorUser) Collect(ctx context.Context, callback chan<- func()) {
	listOpts := pagerduty.ListUsersOptions{}
	listOpts.Limit = PAGERDUTY_LIST_LIMIT
	listOpts.Offset = 0

	if len(m.teamListOpt) > 0 {
		listOpts.TeamIDs = m.teamListOpt
	}

	userMetricList := MetricCollectorList{}

	for {
		Logger.Verbosef(" - fetch users (offset: %v, limit:%v)", listOpts.Offset, listOpts.Limit)

		list, err := PagerDutyClient.ListUsers(listOpts)
		m.CollectorReference.PrometheusApiCounter().WithLabelValues("ListUsers").Inc()

		if err != nil {
			panic(err)
		}

		for _, user := range list.Users {
			userMetricList.AddInfo(prometheus.Labels{
				"userID":       user.ID,
				"userName":     user.Name,
				"userMail":     user.Email,
				"userAvatar":   user.AvatarURL,
				"userColor":    user.Color,
				"userJobTitle": user.JobTitle,
				"userRole":     user.Role,
				"userTimezone": user.Timezone,
			})
		}

		listOpts.Offset += list.Limit
		if !list.More {
			break
		}
	}

	// set metrics
	callback <- func() {
		userMetricList.GaugeSet(m.prometheus.user)
	}
}

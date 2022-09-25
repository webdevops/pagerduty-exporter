package main

import (
	"github.com/PagerDuty/go-pagerduty"
	"github.com/prometheus/client_golang/prometheus"
	prometheusCommon "github.com/webdevops/go-common/prometheus"
	"github.com/webdevops/go-common/prometheus/collector"
)

type MetricsCollectorUser struct {
	collector.Processor

	prometheus struct {
		user *prometheus.GaugeVec
	}

	teamListOpt []string
}

func (m *MetricsCollectorUser) Setup(collector *collector.Collector) {
	m.Processor.Setup(collector)

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

func (m *MetricsCollectorUser) Collect(callback chan<- func()) {
	listOpts := pagerduty.ListUsersOptions{}
	listOpts.Limit = PagerdutyListLimit
	listOpts.Offset = 0

	if len(m.teamListOpt) > 0 {
		listOpts.TeamIDs = m.teamListOpt
	}

	userMetricList := prometheusCommon.NewMetricsList()

	for {
		m.Logger().Debugf("fetch users (offset: %v, limit:%v)", listOpts.Offset, listOpts.Limit)

		list, err := PagerDutyClient.ListUsersWithContext(m.Context(), listOpts)
		PrometheusPagerDutyApiCounter.WithLabelValues("ListUsers").Inc()

		if err != nil {
			m.Logger().Panic(err)
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

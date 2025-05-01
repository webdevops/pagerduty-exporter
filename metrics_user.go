package main

import (
	"github.com/PagerDuty/go-pagerduty"
	"github.com/prometheus/client_golang/prometheus"
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
	m.Collector.RegisterMetricList("pagerduty_user_info", m.prometheus.user, true)
}

func (m *MetricsCollectorUser) Reset() {
}

func (m *MetricsCollectorUser) Collect(callback chan<- func()) {
	listOpts := pagerduty.ListUsersOptions{}
	listOpts.Limit = PagerdutyListLimit
	listOpts.Offset = 0

	if len(m.teamListOpt) > 0 {
		listOpts.TeamIDs = m.teamListOpt
	}

	userMetricList := m.Collector.GetMetricList("pagerduty_user_info")

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
		if stopPagerdutyPaging(list.APIListObject) {
			break
		}
	}
}

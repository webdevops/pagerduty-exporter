package main

import (
	"github.com/PagerDuty/go-pagerduty"
	"github.com/prometheus/client_golang/prometheus"
	prometheusCommon "github.com/webdevops/go-common/prometheus"
	"github.com/webdevops/go-common/prometheus/collector"
)

type MetricsCollectorTeam struct {
	collector.Processor

	prometheus struct {
		team *prometheus.GaugeVec
	}
}

func (m *MetricsCollectorTeam) Setup(collector *collector.Collector) {
	m.Processor.Setup(collector)

	m.prometheus.team = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pagerduty_team_info",
			Help: "PagerDuty team",
		},
		[]string{
			"teamID",
			"teamName",
			"teamUrl",
		},
	)

	prometheus.MustRegister(m.prometheus.team)
}

func (m *MetricsCollectorTeam) Reset() {
	m.prometheus.team.Reset()
}

func (m *MetricsCollectorTeam) Collect(callback chan<- func()) {
	listOpts := pagerduty.ListTeamOptions{}
	listOpts.Limit = PagerdutyListLimit
	listOpts.Offset = 0

	teamMetricList := prometheusCommon.NewMetricsList()

	for {
		m.Logger().Debugf("fetch teams (offset: %v, limit:%v)", listOpts.Offset, listOpts.Limit)

		list, err := PagerDutyClient.ListTeamsWithContext(m.Context(), listOpts)
		PrometheusPagerDutyApiCounter.WithLabelValues("ListTeams").Inc()

		if err != nil {
			m.Logger().Panic(err)
		}

		for _, team := range list.Teams {
			teamMetricList.AddInfo(prometheus.Labels{
				"teamID":   team.ID,
				"teamName": team.Name,
				"teamUrl":  team.HTMLURL,
			})
		}

		listOpts.Offset += list.Limit
		if !list.More {
			break
		}
	}

	// set metrics
	callback <- func() {
		teamMetricList.GaugeSet(m.prometheus.team)
	}
}

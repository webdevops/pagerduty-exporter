package main

import (
	"context"
	"github.com/PagerDuty/go-pagerduty"
	"github.com/prometheus/client_golang/prometheus"
)

type MetricsCollectorTeam struct {
	CollectorProcessorGeneral

	prometheus struct {
		team *prometheus.GaugeVec
	}
}

func (m *MetricsCollectorTeam) Setup(collector *CollectorGeneral) {
	m.CollectorReference = collector

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

func (m *MetricsCollectorTeam) Collect(ctx context.Context, callback chan<- func()) {
	listOpts := pagerduty.ListTeamOptions{}
	listOpts.Limit = PagerdutyListLimit
	listOpts.Offset = 0

	teamMetricList := MetricCollectorList{}

	for {
		daemonLogger.Verbosef(" - fetch teams (offset: %v, limit:%v)", listOpts.Offset, listOpts.Limit)

		list, err := PagerDutyClient.ListTeams(listOpts)
		m.CollectorReference.PrometheusAPICounter().WithLabelValues("ListTeams").Inc()

		if err != nil {
			panic(err)
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

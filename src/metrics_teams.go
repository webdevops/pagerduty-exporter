package main

import (
	"github.com/mblaschke/go-pagerduty"
	"github.com/prometheus/client_golang/prometheus"
)

func collectTeams(callback chan<- func()) {
	listOpts := pagerduty.ListTeamOptions{}
	listOpts.Limit = PAGERDUTY_LIST_LIMIT
	listOpts.Offset = 0

	teamList := []prometheusEntry{}

	for {
		Logger.Verbosef(" - fetch teams (offset: %v, limit:%v)", listOpts.Offset, listOpts.Limit)

		list, err := PagerDutyClient.ListTeams(listOpts)
		prometheusApiCounter.WithLabelValues("ListTeams").Inc()

		if err != nil {
			panic(err)
		}

		for _, team := range list.Teams {
			row := prometheusEntry{
				labels: prometheus.Labels{
					"teamID": team.ID,
					"teamName": team.Name,
					"teamUrl": team.HTMLURL,
				},
				value: 1,
			}
			teamList = append(teamList, row)
		}

		listOpts.Offset += list.Limit
		if !list.More {
			break
		}
	}

	// set metrics
	callback <- func() {
		for _, row := range teamList {
			prometheusTeam.With(row.labels).Set(row.value)
		}
	}
}

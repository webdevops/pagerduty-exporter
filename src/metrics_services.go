package main

import (
	"github.com/mblaschke/go-pagerduty"
	"github.com/prometheus/client_golang/prometheus"
)

func collectServices(callback chan<- func()) {
	listOpts := pagerduty.ListServiceOptions{}
	listOpts.Limit = PAGERDUTY_LIST_LIMIT
	listOpts.Offset = 0

	serviceList := []prometheusEntry{}

	for {
		Logger.Verbose(" - fetch services (offset: %v, limit:%v)", listOpts.Offset, listOpts.Limit)

		list, err := PagerDutyClient.ListServices(listOpts)
		prometheusApiCounter.WithLabelValues("ListServices").Inc()


		if err != nil {
			panic(err)
		}

		for _, service := range list.Services {
			for _, team := range service.Teams {
				row := prometheusEntry{
					labels: prometheus.Labels{
						"serviceID": service.ID,
						"teamID": team.ID,
						"serviceName": service.Name,
						"serviceUrl": service.HTMLURL,
					},
					value: 1,
				}
				serviceList = append(serviceList, row)
			}
		}

		listOpts.Offset += list.Limit
		if !list.More {
			break
		}
	}

	// set metrics
	callback <- func() {
		for _, row := range serviceList {
			prometheusService.With(row.labels).Set(row.value)
		}
	}
}

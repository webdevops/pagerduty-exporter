package main

import (
	"context"
	"github.com/PagerDuty/go-pagerduty"
	"github.com/prometheus/client_golang/prometheus"
)

type MetricsCollectorService struct {
	CollectorProcessorGeneral

	prometheus struct {
		service *prometheus.GaugeVec
	}

	teamListOpt []string
}

func (m *MetricsCollectorService) Setup(collector *CollectorGeneral) {
	m.CollectorReference = collector

	m.prometheus.service = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pagerduty_service_info",
			Help: "PagerDuty service",
		},
		[]string{
			"serviceID",
			"teamID",
			"serviceName",
			"serviceUrl",
		},
	)

	prometheus.MustRegister(m.prometheus.service)
}

func (m *MetricsCollectorService) Reset() {
	m.prometheus.service.Reset()
}

func (m *MetricsCollectorService) Collect(ctx context.Context, callback chan<- func()) {
	listOpts := pagerduty.ListServiceOptions{}
	listOpts.Limit = PAGERDUTY_LIST_LIMIT
	listOpts.Offset = 0

	if len(m.teamListOpt) > 0 {
		listOpts.TeamIDs = m.teamListOpt
	}

	serviceMetricList := MetricCollectorList{}

	for {
		Logger.Verbosef(" - fetch services (offset: %v, limit:%v)", listOpts.Offset, listOpts.Limit)

		list, err := PagerDutyClient.ListServices(listOpts)
		m.CollectorReference.PrometheusApiCounter().WithLabelValues("ListServices").Inc()

		if err != nil {
			panic(err)
		}

		for _, service := range list.Services {
			if len(service.Teams) > 0 {
				for _, team := range service.Teams {

					serviceMetricList.AddInfo(prometheus.Labels{
						"serviceID":   service.ID,
						"teamID":      team.ID,
						"serviceName": service.Name,
						"serviceUrl":  service.HTMLURL,
					})
				}
			} else {
				serviceMetricList.AddInfo(prometheus.Labels{
					"serviceID":   service.ID,
					"teamID":      "",
					"serviceName": service.Name,
					"serviceUrl":  service.HTMLURL,
				})
			}
		}

		listOpts.Offset += list.Limit
		if !list.More {
			break
		}
	}

	// set metrics
	callback <- func() {
		serviceMetricList.GaugeSet(m.prometheus.service)
	}
}

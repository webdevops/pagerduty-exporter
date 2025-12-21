package main

import (
	"log/slog"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/webdevops/go-common/prometheus/collector"
)

type MetricsCollectorService struct {
	collector.Processor

	prometheus struct {
		service *prometheus.GaugeVec
	}

	teamListOpt []string
}

func (m *MetricsCollectorService) Setup(collector *collector.Collector) {
	m.Processor.Setup(collector)

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
	m.Collector.RegisterMetricList("pagerduty_service_info", m.prometheus.service, true)
}

func (m *MetricsCollectorService) Reset() {
}

func (m *MetricsCollectorService) Collect(callback chan<- func()) {
	listOpts := pagerduty.ListServiceOptions{}
	listOpts.Limit = PagerdutyListLimit
	listOpts.Offset = 0

	if len(m.teamListOpt) > 0 {
		listOpts.TeamIDs = m.teamListOpt
	}

	serviceMetricList := m.Collector.GetMetricList("pagerduty_service_info")

	for {
		m.Logger().Debug("fetch services ", slog.Uint64("offset", uint64(listOpts.Offset)), slog.Uint64("limit", uint64(listOpts.Limit)))

		list, err := PagerDutyClient.ListServicesWithContext(m.Context(), listOpts)
		PrometheusPagerDutyApiCounter.WithLabelValues("ListServices").Inc()

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
		if stopPagerdutyPaging(list.APIListObject) {
			break
		}
	}
}

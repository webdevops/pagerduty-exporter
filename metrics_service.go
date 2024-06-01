package main

import (
	"github.com/PagerDuty/go-pagerduty"
	"github.com/prometheus/client_golang/prometheus"
	prometheusCommon "github.com/webdevops/go-common/prometheus"
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

	prometheus.MustRegister(m.prometheus.service)
}

func (m *MetricsCollectorService) Reset() {
	m.prometheus.service.Reset()
}

func (m *MetricsCollectorService) Collect(callback chan<- func()) {
	listOpts := pagerduty.ListServiceOptions{}
	listOpts.Limit = PagerdutyListLimit
	listOpts.Offset = 0

	if len(m.teamListOpt) > 0 {
		listOpts.TeamIDs = m.teamListOpt
	}

	serviceMetricList := prometheusCommon.NewMetricsList()

	for {
		m.Logger().Debugf("fetch services (offset: %v, limit:%v)", listOpts.Offset, listOpts.Limit)

		list, err := PagerDutyClient.ListServicesWithContext(m.Context(), listOpts)
		PrometheusPagerDutyApiCounter.WithLabelValues("ListServices").Inc()

		if err != nil {
			m.Logger().Panic(err)
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

	// set metrics
	callback <- func() {
		serviceMetricList.GaugeSet(m.prometheus.service)
	}
}

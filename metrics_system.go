package main

import (
	"encoding/json"
	"fmt"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/webdevops/go-common/prometheus/collector"
)

type MetricsCollectorSystem struct {
	collector.Processor

	prometheus struct {
		license                     *prometheus.GaugeVec
		licenseCurrent              *prometheus.GaugeVec
		licenseAllocationsAvailable *prometheus.GaugeVec
	}
}

func (m *MetricsCollectorSystem) Setup(collector *collector.Collector) {
	m.Processor.Setup(collector)

	m.prometheus.license = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pagerduty_system_license_info",
			Help: "PagerDuty license",
		},
		[]string{
			"licenseID",
			"licenseType",
			"licenseName",
		},
	)
	m.Collector.RegisterMetricList("pagerduty_system_license", m.prometheus.license, true)

	m.prometheus.licenseCurrent = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pagerduty_system_license_current",
			Help: "PagerDuty license current value",
		},
		[]string{
			"licenseID",
			"licenseType",
			"licenseName",
		},
	)
	m.Collector.RegisterMetricList("pagerduty_system_licenses_current", m.prometheus.licenseCurrent, true)

	m.prometheus.licenseAllocationsAvailable = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pagerduty_system_license_allocations_available",
			Help: "PagerDuty license allocations available",
		},
		[]string{
			"licenseID",
			"licenseType",
			"licenseName",
		},
	)
	m.Collector.RegisterMetricList("pagerduty_system_license_allocations_available", m.prometheus.licenseAllocationsAvailable, true)
}

func (m *MetricsCollectorSystem) Reset() {
}

func (m *MetricsCollectorSystem) Collect(callback chan<- func()) {
	listOpts := pagerduty.ListServiceOptions{}
	listOpts.Limit = PagerdutyListLimit
	listOpts.Offset = 0

	licenseMetricList := m.Collector.GetMetricList("pagerduty_system_license")
	licenseCurrentMetricList := m.Collector.GetMetricList("pagerduty_system_licenses_current")
	licenseAllocationsAvailableMetricList := m.Collector.GetMetricList("pagerduty_system_license_allocations_available")

	resp, err := PagerDutyClient.ListLicensesWithContext(m.Context())
	if err != nil {
		m.Logger().Panic(err)
	}

	for _, license := range resp.Licenses {
		foo, _ := json.Marshal(license)
		fmt.Println(string(foo))
		licenseMetricList.AddInfo(prometheus.Labels{
			"licenseID":   license.ID,
			"licenseType": license.Type,
			"licenseName": license.Name,
		})

		licenseCurrentMetricList.Add(prometheus.Labels{
			"licenseID":   license.ID,
			"licenseType": license.Type,
			"licenseName": license.Name,
		}, float64(license.CurrentValue))

		licenseAllocationsAvailableMetricList.Add(prometheus.Labels{
			"licenseID":   license.ID,
			"licenseType": license.Type,
			"licenseName": license.Name,
		}, float64(license.AllocationsAvailable))
	}
}

package main

import (
	"time"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/webdevops/go-common/prometheus/collector"
)

type MetricsCollectorMaintenanceWindow struct {
	collector.Processor

	prometheus struct {
		maintenanceWindow       *prometheus.GaugeVec
		maintenanceWindowStatus *prometheus.GaugeVec
	}

	teamListOpt []string
}

func (m *MetricsCollectorMaintenanceWindow) Setup(collector *collector.Collector) {
	m.Processor.Setup(collector)

	m.prometheus.maintenanceWindow = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pagerduty_maintenancewindow_info",
			Help: "PagerDuty MaintenanceWindow",
		},
		[]string{
			"windowID",
			"serviceID",
		},
	)
	m.Collector.RegisterMetricList("pagerduty_maintenancewindow_info", m.prometheus.maintenanceWindow, true)

	m.prometheus.maintenanceWindowStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pagerduty_maintenancewindow_status",
			Help: "PagerDuty MaintenanceWindow",
		},
		[]string{
			"windowID",
			"serviceID",
			"type",
		},
	)
	m.Collector.RegisterMetricList("pagerduty_maintenancewindow_status", m.prometheus.maintenanceWindowStatus, true)
}

func (m *MetricsCollectorMaintenanceWindow) Reset() {
}

func (m *MetricsCollectorMaintenanceWindow) Collect(callback chan<- func()) {
	listOpts := pagerduty.ListMaintenanceWindowsOptions{}
	listOpts.Limit = PagerdutyListLimit
	listOpts.Offset = 0

	if len(m.teamListOpt) > 0 {
		listOpts.TeamIDs = m.teamListOpt
	}

	maintWindowMetricList := m.Collector.GetMetricList("pagerduty_maintenancewindow_info")
	maintWindowsStatusMetricList := m.Collector.GetMetricList("pagerduty_maintenancewindow_status")

	for {
		m.Logger().Debugf("fetch maintenance windows (offset: %v, limit:%v)", listOpts.Offset, listOpts.Limit)

		list, err := PagerDutyClient.ListMaintenanceWindowsWithContext(m.Context(), listOpts)
		PrometheusPagerDutyApiCounter.WithLabelValues("ListMaintenanceWindows").Inc()

		if err != nil {
			m.Logger().Panic(err)
		}

		currentTime := time.Now()

		for _, maintWindow := range list.MaintenanceWindows {
			startTime, _ := time.Parse(time.RFC3339, maintWindow.StartTime)
			endTime, _ := time.Parse(time.RFC3339, maintWindow.EndTime)

			if endTime.Before(currentTime) {
				continue
			}

			for _, service := range maintWindow.Services {
				maintWindowMetricList.AddInfo(prometheus.Labels{
					"serviceID": service.ID,
					"windowID":  maintWindow.ID,
				})

				maintWindowsStatusMetricList.AddTime(prometheus.Labels{
					"windowID":  service.ID,
					"serviceID": service.ID,
					"type":      "startTime",
				}, startTime)

				maintWindowsStatusMetricList.AddTime(prometheus.Labels{
					"windowID":  service.ID,
					"serviceID": service.ID,
					"type":      "endTime",
				}, endTime)
			}
		}

		listOpts.Offset += list.Limit
		if stopPagerdutyPaging(list.APIListObject) {
			break
		}
	}
}

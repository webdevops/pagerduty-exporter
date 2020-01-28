package main

import (
	"context"
	"github.com/PagerDuty/go-pagerduty"
	"github.com/prometheus/client_golang/prometheus"
	"time"
)

type MetricsCollectorMaintenanceWindow struct {
	CollectorProcessorGeneral

	prometheus struct {
		maintenanceWindow       *prometheus.GaugeVec
		maintenanceWindowStatus *prometheus.GaugeVec
	}

	teamListOpt []string
}

func (m *MetricsCollectorMaintenanceWindow) Setup(collector *CollectorGeneral) {
	m.CollectorReference = collector

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

	prometheus.MustRegister(m.prometheus.maintenanceWindow)
	prometheus.MustRegister(m.prometheus.maintenanceWindowStatus)
}

func (m *MetricsCollectorMaintenanceWindow) Reset() {
	m.prometheus.maintenanceWindow.Reset()
	m.prometheus.maintenanceWindowStatus.Reset()
}

func (m *MetricsCollectorMaintenanceWindow) Collect(ctx context.Context, callback chan<- func()) {
	listOpts := pagerduty.ListMaintenanceWindowsOptions{}
	listOpts.Limit = PAGERDUTY_LIST_LIMIT
	listOpts.Offset = 0

	if len(m.teamListOpt) > 0 {
		listOpts.TeamIDs = m.teamListOpt
	}

	maintWindowMetricList := MetricCollectorList{}
	maintWindowsStatusMetricList := MetricCollectorList{}

	for {
		Logger.Verbosef(" - fetch maintenance windows (offset: %v, limit:%v)", listOpts.Offset, listOpts.Limit)

		list, err := PagerDutyClient.ListMaintenanceWindows(listOpts)
		m.CollectorReference.PrometheusApiCounter().WithLabelValues("ListMaintenanceWindows").Inc()

		if err != nil {
			panic(err)
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
		if !list.More {
			break
		}
	}

	// set metrics
	callback <- func() {
		maintWindowMetricList.GaugeSet(m.prometheus.maintenanceWindow)
		maintWindowsStatusMetricList.GaugeSet(m.prometheus.maintenanceWindowStatus)
	}
}

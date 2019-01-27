package main

import (
	"time"
	"github.com/mblaschke/go-pagerduty"
	"github.com/prometheus/client_golang/prometheus"
)

func collectMaintenanceWindows(callback chan<- func()) {
	listOpts := pagerduty.ListMaintenanceWindowsOptions{}
	listOpts.Limit = PAGERDUTY_LIST_LIMIT
	listOpts.Offset = 0

	maintenanceWindowList := []prometheusEntry{}
	maintenanceWindowStatusList := []prometheusEntry{}

	for {
		Logger.Verbosef(" - fetch maintenance windows (offset: %v, limit:%v)", listOpts.Offset, listOpts.Limit)

		list, err := PagerDutyClient.ListMaintenanceWindows(listOpts)
		prometheusApiCounter.WithLabelValues("ListMaintenanceWindows").Inc()

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
				row := prometheusEntry{
					labels: prometheus.Labels{
						"serviceID": service.ID,
						"windowID": maintWindow.ID,
					},
					value: 1,
				}
				maintenanceWindowList = append(maintenanceWindowList, row)


				rowStart := prometheusEntry{
					labels: prometheus.Labels{
						"windowID": service.ID,
						"serviceID": service.ID,
						"type": "startTime",
					},
					value: float64(startTime.Unix()),
				}

				rowEnd := prometheusEntry{
					labels: prometheus.Labels{
						"windowID": service.ID,
						"serviceID": service.ID,
						"type": "endTime",
					},
					value: float64(endTime.Unix()),
				}

				maintenanceWindowStatusList = append(maintenanceWindowStatusList, rowStart, rowEnd)
			}
		}

		listOpts.Offset += list.Limit
		if !list.More {
			break
		}
	}

	// set metrics
	callback <- func() {
		for _, row := range maintenanceWindowList {
			prometheusMaintenanceWindows.With(row.labels).Set(row.value)
		}

		for _, row := range maintenanceWindowStatusList {
			prometheusMaintenanceWindowsStatus.With(row.labels).Set(row.value)
		}
	}
}

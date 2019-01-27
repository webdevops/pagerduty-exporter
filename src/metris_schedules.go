package main

import (
	"time"
	"github.com/mblaschke/go-pagerduty"
	"github.com/prometheus/client_golang/prometheus"
)

func collectSchedules(callback chan<- func()) {
	listOpts := pagerduty.ListSchedulesOptions{}
	listOpts.Limit = PAGERDUTY_LIST_LIMIT
	listOpts.Offset = 0

	scheduleList := []prometheusEntry{}

	for {
		Logger.Verbosef(" - fetch schedules (offset: %v, limit:%v)", listOpts.Offset, listOpts.Limit)

		list, err := PagerDutyClient.ListSchedules(listOpts)
		prometheusApiCounter.WithLabelValues("ListSchedules").Inc()

		if err != nil {
			panic(err)
		}

		for _, schedule := range list.Schedules {
			row := prometheusEntry{
				labels: prometheus.Labels{
					"scheduleID": schedule.ID,
					"scheduleName": schedule.Name,
					"scheduleTimeZone": schedule.TimeZone,
				},
				value: 1,
			}
			scheduleList = append(scheduleList, row)

			collectScheduleInformation(schedule.ID, callback)
			collectScheduleOverrides(schedule.ID, callback)
		}

		listOpts.Offset += list.Limit
		if !list.More {
			break
		}
	}

	// set metrics
	callback <- func() {
		for _, row := range scheduleList {
			prometheusSchedule.With(row.labels).Set(row.value)
		}
	}
}

func collectScheduleOnCalls(callback chan<- func()) {
	listOpts := pagerduty.ListOnCallOptions{}
	listOpts.Limit = PAGERDUTY_LIST_LIMIT
	listOpts.Earliest = true
	listOpts.Offset = 0

	onCallList := []prometheusEntry{}

	for {
		Logger.Verbosef(" - fetch schedule oncalls (offset: %v, limit:%v)", listOpts.Offset, listOpts.Limit)

		list, err := PagerDutyClient.ListOnCalls(listOpts)
		prometheusApiCounter.WithLabelValues("ListOnCalls").Inc()

		if err != nil {
			panic(err)
		}

		for _, oncall := range list.OnCalls {
			startTime, _ := time.Parse(time.RFC3339, oncall.Start)
			endTime, _ := time.Parse(time.RFC3339, oncall.End)

			startValue := float64(startTime.Unix())
			endValue := float64(endTime.Unix())

			if startValue < 0 {
				startValue = 1
			}

			if endValue < 0 {
				endValue = 1
			}

			// start
			rowStart := prometheusEntry{
				labels: prometheus.Labels{
					"scheduleID": oncall.Schedule.ID,
					"userID": oncall.User.ID,
					"escalationLevel": uintToString(oncall.EscalationLevel),
					"type": "startTime",
				},
				value: startValue,
			}

			// end
			rowEnd := prometheusEntry{
				labels: prometheus.Labels{
					"scheduleID": oncall.Schedule.ID,
					"userID": oncall.User.ID,
					"escalationLevel": uintToString(oncall.EscalationLevel),
					"type": "endTime",
				},
				value: endValue,
			}

			onCallList = append(onCallList, rowStart, rowEnd)
		}

		// loop
		listOpts.Offset += list.Limit
		if !list.More {
			break
		}
	}

	// set metrics
	callback <- func() {
		for _, row := range onCallList {
			prometheusScheduleOnCall.With(row.labels).Set(row.value)
		}
	}
}


func collectScheduleInformation(scheduleId string, callback chan<- func()) {
	filterSince := time.Now().Add(-opts.ScrapeTime)
	filterUntil := time.Now().Add(opts.PagerDutyScheduleEntryTimeframe)

	listOpts := pagerduty.GetScheduleOptions{}
	listOpts.Limit = PAGERDUTY_LIST_LIMIT
	listOpts.Since = filterSince.Format(time.RFC3339)
	listOpts.Until = filterUntil.Format(time.RFC3339)
	listOpts.Offset = 0

	Logger.Verbosef(" - fetch schedule information (schedule: %v, offset: %v, limit:%v)", scheduleId, listOpts.Offset, listOpts.Limit)

	schedule, err := PagerDutyClient.GetSchedule(scheduleId, listOpts)
	prometheusApiCounter.WithLabelValues("GetSchedule").Inc()

	if err != nil {
		panic(err)
	}

	scheduleLayerList := []prometheusEntry{}
	scheduleLayerEntryList := []prometheusEntry{}
	scheduleLayerCoverageList := []prometheusEntry{}
	scheduleFinalEntryList := []prometheusEntry{}

	for _, scheduleLayer := range schedule.ScheduleLayers {

		// schedule layer informations
		scheduleLayerRow := prometheusEntry{
			labels: prometheus.Labels{
				"scheduleID": scheduleId,
				"scheduleLayerID": scheduleLayer.ID,
				"scheduleLayerName": scheduleLayer.Name,
			},
			value: 1,
		}
		scheduleLayerList = append(scheduleLayerList, scheduleLayerRow)

		// schedule layer entries
		for _, scheduleEntry := range scheduleLayer.RenderedScheduleEntries {
			startTime, _ := time.Parse(time.RFC3339, scheduleEntry.Start)
			endTime, _ := time.Parse(time.RFC3339, scheduleEntry.End)

			// schedule item start
			rowStart := prometheusEntry{
				labels: prometheus.Labels{
					"scheduleID": scheduleId,
					"scheduleLayerID": scheduleLayer.ID,
					"userID": scheduleEntry.User.ID,
					"time": startTime.Format(opts.PagerDutyScheduleEntryTimeFormat),
					"type": "startTime",
				},
				value: float64(startTime.Unix()),
			}

			// schedule item end
			rowEnd := prometheusEntry{
				labels: prometheus.Labels{
					"scheduleID": scheduleId,
					"scheduleLayerID": scheduleLayer.ID,
					"userID": scheduleEntry.User.ID,
					"time": endTime.Format(opts.PagerDutyScheduleEntryTimeFormat),
					"type": "endTime",
				},
				value: float64(endTime.Unix()),
			}

			scheduleLayerEntryList = append(scheduleLayerEntryList, rowStart, rowEnd)
		}

		// layer coverage
		rowCoverage := prometheusEntry{
			value: scheduleLayer.RenderedCoveragePercentage,
			labels: prometheus.Labels{
				"scheduleID": scheduleId,
				"scheduleLayerID": scheduleLayer.ID,
			},
		}

		scheduleLayerCoverageList = append(scheduleLayerCoverageList, rowCoverage)
	}


	// final schedule entries
	for _, scheduleEntry := range schedule.FinalSchedule.RenderedScheduleEntries {
		startTime, _ := time.Parse(time.RFC3339, scheduleEntry.Start)
		endTime, _ := time.Parse(time.RFC3339, scheduleEntry.End)

		// schedule item start
		rowStart := prometheusEntry{
			labels: prometheus.Labels{
				"scheduleID": scheduleId,
				"userID": scheduleEntry.User.ID,
				"time": startTime.Format(opts.PagerDutyScheduleEntryTimeFormat),
				"type": "startTime",
			},
			value: float64(startTime.Unix()),
		}

		// schedule item end
		rowEnd := prometheusEntry{
			labels: prometheus.Labels{
				"scheduleID": scheduleId,
				"userID": scheduleEntry.User.ID,
				"time": endTime.Format(opts.PagerDutyScheduleEntryTimeFormat),
				"type": "endTime",
			},
			value: float64(endTime.Unix()),
		}

		scheduleFinalEntryList = append(scheduleFinalEntryList, rowStart, rowEnd)
	}

	// final schedule coverage
	scheduleFinalCoverageLabels := prometheus.Labels{
		"scheduleID": scheduleId,
	}
	scheduleFinalCoverageValue := schedule.FinalSchedule.RenderedCoveragePercentage

	// set metrics
	callback <- func() {
		// layer schedule
		for _, row := range scheduleLayerList {
			prometheusScheduleLayer.With(row.labels).Set(row.value)
		}

		for _, row := range scheduleLayerCoverageList {
			prometheusScheduleLayerCoverage.With(row.labels).Set(row.value)
		}

		for _, row := range scheduleLayerEntryList {
			prometheusScheduleLayerEntry.With(row.labels).Set(row.value)
		}

		// final schedule
		prometheusScheduleFinalCoverage.With(scheduleFinalCoverageLabels).Set(scheduleFinalCoverageValue)
		for _, row := range scheduleFinalEntryList {
			prometheusScheduleFinalEntry.With(row.labels).Set(row.value)
		}
	}
}

func collectScheduleOverrides(scheduleId string, callback chan<- func()) {
	filterSince := time.Now().Add(-opts.ScrapeTime)
	filterUntil := time.Now().Add(opts.PagerDutyScheduleOverrideTimeframe)

	listOpts := pagerduty.ListOverridesOptions{}
	listOpts.Limit = PAGERDUTY_LIST_LIMIT
	listOpts.Since = filterSince.Format(time.RFC3339)
	listOpts.Until = filterUntil.Format(time.RFC3339)
	listOpts.Offset = 0

	overrideList := []prometheusEntry{}

	for {
		Logger.Verbosef(" - fetch schedule overrides (schedule: %v, offset: %v, limit:%v)", scheduleId, listOpts.Offset, listOpts.Limit)

		list, err := PagerDutyClient.ListOverrides(scheduleId, listOpts)
		prometheusApiCounter.WithLabelValues("ListOverrides").Inc()

		if err != nil {
			panic(err)
		}

		for _, override := range list.Overrides {
			startTime, _ := time.Parse(time.RFC3339, override.Start)
			endTime, _ := time.Parse(time.RFC3339, override.End)

			rowStart := prometheusEntry{
				labels: prometheus.Labels{
					"overrideID": override.ID,
					"scheduleID": scheduleId,
					"userID": override.User.ID,
					"type": "startTime",
				},
				value: float64(startTime.Unix()),
			}

			rowEnd := prometheusEntry{
				labels: prometheus.Labels{
					"overrideID": override.ID,
					"scheduleID": scheduleId,
					"userID": override.User.ID,
					"type": "endTime",
				},
				value: float64(endTime.Unix()),
			}

			overrideList = append(overrideList, rowStart, rowEnd)
		}

		listOpts.Offset += list.Limit
		if !list.More {
			break
		}
	}

	// set metrics
	callback <- func() {
		for _, row := range overrideList {
			prometheusScheduleOverwrite.With(row.labels).Set(row.value)
		}
	}
}

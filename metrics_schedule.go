package main

import (
	"time"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/webdevops/go-common/prometheus/collector"
)

type MetricsCollectorSchedule struct {
	collector.Processor

	prometheus struct {
		schedule              *prometheus.GaugeVec
		scheduleLayer         *prometheus.GaugeVec
		scheduleLayerEntry    *prometheus.GaugeVec
		scheduleLayerCoverage *prometheus.GaugeVec
		scheduleFinalEntry    *prometheus.GaugeVec
		scheduleFinalCoverage *prometheus.GaugeVec
		scheduleOnCall        *prometheus.GaugeVec
		scheduleOverwrite     *prometheus.GaugeVec
	}
}

func (m *MetricsCollectorSchedule) Setup(collector *collector.Collector) {
	m.Processor.Setup(collector)

	m.prometheus.schedule = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pagerduty_schedule_info",
			Help: "PagerDuty schedule",
		},
		[]string{"scheduleID", "scheduleName", "scheduleTimeZone"},
	)
	m.Collector.RegisterMetricList("pagerduty_schedule_info", m.prometheus.schedule, true)

	m.prometheus.scheduleLayer = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pagerduty_schedule_layer_info",
			Help: "PagerDuty schedule layer information",
		},
		[]string{"scheduleID", "scheduleLayerID", "scheduleLayerName"},
	)
	m.Collector.RegisterMetricList("pagerduty_schedule_layer_info", m.prometheus.scheduleLayer, true)

	m.prometheus.scheduleLayerEntry = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pagerduty_schedule_layer_entry",
			Help: "PagerDuty schedule layer entries",
		},
		[]string{"scheduleLayerID", "scheduleID", "userID", "time", "type"},
	)
	m.Collector.RegisterMetricList("pagerduty_schedule_layer_entry", m.prometheus.scheduleLayerEntry, true)

	m.prometheus.scheduleLayerCoverage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pagerduty_schedule_layer_coverage",
			Help: "PagerDuty schedule layer entry coverage",
		},
		[]string{"scheduleLayerID", "scheduleID"},
	)
	m.Collector.RegisterMetricList("pagerduty_schedule_layer_coverage", m.prometheus.scheduleLayerCoverage, true)

	m.prometheus.scheduleFinalEntry = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pagerduty_schedule_final_entry",
			Help: "PagerDuty schedule final entries",
		},
		[]string{"scheduleID", "userID", "time", "type"},
	)
	m.Collector.RegisterMetricList("pagerduty_schedule_final_entry", m.prometheus.scheduleFinalEntry, true)

	m.prometheus.scheduleFinalCoverage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pagerduty_schedule_final_coverage",
			Help: "PagerDuty schedule final entry coverage",
		},
		[]string{"scheduleID"},
	)
	m.Collector.RegisterMetricList("pagerduty_schedule_final_coverage", m.prometheus.scheduleFinalCoverage, true)

	m.prometheus.scheduleOverwrite = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pagerduty_schedule_override",
			Help: "PagerDuty schedule override",
		},
		[]string{"overrideID", "scheduleID", "userID", "type"},
	)
	m.Collector.RegisterMetricList("pagerduty_schedule_override", m.prometheus.scheduleOverwrite, true)
}

func (m *MetricsCollectorSchedule) Reset() {
}

func (m *MetricsCollectorSchedule) Collect(callback chan<- func()) {
	listOpts := pagerduty.ListSchedulesOptions{}
	listOpts.Limit = PagerdutyListLimit
	listOpts.Offset = 0

	scheduleMetricList := m.Collector.GetMetricList("pagerduty_schedule_info")

	for {
		m.Logger().Debugf("fetch schedules (offset: %v, limit:%v)", listOpts.Offset, listOpts.Limit)

		list, err := PagerDutyClient.ListSchedulesWithContext(m.Context(), listOpts)
		PrometheusPagerDutyApiCounter.WithLabelValues("ListSchedules").Inc()

		if err != nil {
			m.Logger().Panic(err)
		}

		for _, schedule := range list.Schedules {
			scheduleMetricList.AddInfo(prometheus.Labels{
				"scheduleID":       schedule.ID,
				"scheduleName":     schedule.Name,
				"scheduleTimeZone": schedule.TimeZone,
			})

			// get detail information about schedule
			m.collectScheduleInformation(schedule.ID, callback)
			m.collectScheduleOverrides(schedule.ID, callback)
		}

		listOpts.Offset += list.Limit
		if stopPagerdutyPaging(list.APIListObject) {
			break
		}
	}
}

func (m *MetricsCollectorSchedule) collectScheduleInformation(scheduleID string, callback chan<- func()) {
	filterSince := time.Now().Add(-Opts.ScrapeTime.General)
	filterUntil := time.Now().Add(Opts.PagerDuty.Schedule.EntryTimeframe)

	listOpts := pagerduty.GetScheduleOptions{}
	listOpts.Since = filterSince.Format(time.RFC3339)
	listOpts.Until = filterUntil.Format(time.RFC3339)

	m.Logger().Debugf("fetch schedule information (schedule: %v)", scheduleID)

	schedule, err := PagerDutyClient.GetScheduleWithContext(m.Context(), scheduleID, listOpts)
	PrometheusPagerDutyApiCounter.WithLabelValues("GetSchedule").Inc()

	if err != nil {
		m.Logger().Panic(err)
	}

	scheduleLayerMetricList := m.Collector.GetMetricList("pagerduty_schedule_layer_info")
	scheduleLayerEntryMetricList := m.Collector.GetMetricList("pagerduty_schedule_layer_entry")
	scheduleLayerCoverageMetricList := m.Collector.GetMetricList("pagerduty_schedule_layer_coverage")
	scheduleFinalEntryMetricList := m.Collector.GetMetricList("pagerduty_schedule_final_entry")
	scheduleFinalCoverageMetricList := m.Collector.GetMetricList("pagerduty_schedule_final_coverage")

	for _, scheduleLayer := range schedule.ScheduleLayers {

		// schedule layer information
		scheduleLayerMetricList.AddInfo(prometheus.Labels{
			"scheduleID":        scheduleID,
			"scheduleLayerID":   scheduleLayer.ID,
			"scheduleLayerName": scheduleLayer.Name,
		})

		// schedule layer entries
		for _, scheduleEntry := range scheduleLayer.RenderedScheduleEntries {
			startTime, _ := time.Parse(time.RFC3339, scheduleEntry.Start)
			endTime, _ := time.Parse(time.RFC3339, scheduleEntry.End)

			// schedule item start
			scheduleLayerEntryMetricList.AddTime(prometheus.Labels{
				"scheduleID":      scheduleID,
				"scheduleLayerID": scheduleLayer.ID,
				"userID":          scheduleEntry.User.ID,
				"time":            startTime.Format(Opts.PagerDuty.Schedule.EntryTimeFormat),
				"type":            "startTime",
			}, startTime)

			// schedule item end
			scheduleLayerEntryMetricList.AddTime(prometheus.Labels{
				"scheduleID":      scheduleID,
				"scheduleLayerID": scheduleLayer.ID,
				"userID":          scheduleEntry.User.ID,
				"time":            endTime.Format(Opts.PagerDuty.Schedule.EntryTimeFormat),
				"type":            "endTime",
			}, endTime)
		}

		// layer coverage
		scheduleLayerCoverageMetricList.Add(prometheus.Labels{
			"scheduleID":      scheduleID,
			"scheduleLayerID": scheduleLayer.ID,
		}, scheduleLayer.RenderedCoveragePercentage)
	}

	// final schedule entries
	for _, scheduleEntry := range schedule.FinalSchedule.RenderedScheduleEntries {
		startTime, _ := time.Parse(time.RFC3339, scheduleEntry.Start)
		endTime, _ := time.Parse(time.RFC3339, scheduleEntry.End)

		// schedule item start
		scheduleFinalEntryMetricList.AddTime(prometheus.Labels{
			"scheduleID": scheduleID,
			"userID":     scheduleEntry.User.ID,
			"time":       startTime.Format(Opts.PagerDuty.Schedule.EntryTimeFormat),
			"type":       "startTime",
		}, startTime)

		// schedule item end
		scheduleFinalEntryMetricList.AddTime(prometheus.Labels{
			"scheduleID": scheduleID,
			"userID":     scheduleEntry.User.ID,
			"time":       endTime.Format(Opts.PagerDuty.Schedule.EntryTimeFormat),
			"type":       "endTime",
		}, endTime)
	}

	// final schedule coverage
	scheduleFinalCoverageMetricList.Add(prometheus.Labels{
		"scheduleID": scheduleID,
	}, schedule.FinalSchedule.RenderedCoveragePercentage)
}

func (m *MetricsCollectorSchedule) collectScheduleOverrides(scheduleID string, callback chan<- func()) {
	filterSince := time.Now().Add(-Opts.ScrapeTime.General)
	filterUntil := time.Now().Add(Opts.PagerDuty.Schedule.OverrideTimeframe)

	listOpts := pagerduty.ListOverridesOptions{}
	listOpts.Since = filterSince.Format(time.RFC3339)
	listOpts.Until = filterUntil.Format(time.RFC3339)

	overrideMetricList := m.Collector.GetMetricList("pagerduty_schedule_override")

	m.Logger().Debugf("fetch schedule overrides (schedule: %v)", scheduleID)

	list, err := PagerDutyClient.ListOverridesWithContext(m.Context(), scheduleID, listOpts)
	PrometheusPagerDutyApiCounter.WithLabelValues("ListOverrides").Inc()

	if err != nil {
		m.Logger().Panic(err)
	}

	for _, override := range list.Overrides {
		startTime, _ := time.Parse(time.RFC3339, override.Start)
		endTime, _ := time.Parse(time.RFC3339, override.End)

		overrideMetricList.AddTime(prometheus.Labels{
			"overrideID": override.ID,
			"scheduleID": scheduleID,
			"userID":     override.User.ID,
			"type":       "startTime",
		}, startTime)

		overrideMetricList.AddTime(prometheus.Labels{
			"overrideID": override.ID,
			"scheduleID": scheduleID,
			"userID":     override.User.ID,
			"type":       "endTime",
		}, endTime)
	}
}

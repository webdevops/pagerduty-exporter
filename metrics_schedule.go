package main

import (
	"context"
	"github.com/PagerDuty/go-pagerduty"
	"github.com/prometheus/client_golang/prometheus"
	"sync"
	"time"
)

type MetricsCollectorSchedule struct {
	CollectorProcessorGeneral

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

func (m *MetricsCollectorSchedule) Setup(collector *CollectorGeneral) {
	m.CollectorReference = collector

	m.prometheus.schedule = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pagerduty_schedule_info",
			Help: "PagerDuty schedule",
		},
		[]string{"scheduleID", "scheduleName", "scheduleTimeZone"},
	)

	m.prometheus.scheduleLayer = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pagerduty_schedule_layer_info",
			Help: "PagerDuty schedule layer informations",
		},
		[]string{"scheduleID", "scheduleLayerID", "scheduleLayerName"},
	)

	m.prometheus.scheduleLayerEntry = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pagerduty_schedule_layer_entry",
			Help: "PagerDuty schedule layer entries",
		},
		[]string{"scheduleLayerID", "scheduleID", "userID", "time", "type"},
	)

	m.prometheus.scheduleLayerCoverage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pagerduty_schedule_layer_coverage",
			Help: "PagerDuty schedule layer entry coverage",
		},
		[]string{"scheduleLayerID", "scheduleID"},
	)

	m.prometheus.scheduleFinalEntry = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pagerduty_schedule_final_entry",
			Help: "PagerDuty schedule final entries",
		},
		[]string{"scheduleID", "userID", "time", "type"},
	)

	m.prometheus.scheduleFinalCoverage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pagerduty_schedule_final_coverage",
			Help: "PagerDuty schedule final entry coverage",
		},
		[]string{"scheduleID"},
	)

	m.prometheus.scheduleOverwrite = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pagerduty_schedule_override",
			Help: "PagerDuty schedule override",
		},
		[]string{"overrideID", "scheduleID", "userID", "type"},
	)

	prometheus.MustRegister(m.prometheus.schedule)
	prometheus.MustRegister(m.prometheus.scheduleLayer)
	prometheus.MustRegister(m.prometheus.scheduleLayerEntry)
	prometheus.MustRegister(m.prometheus.scheduleLayerCoverage)
	prometheus.MustRegister(m.prometheus.scheduleFinalEntry)
	prometheus.MustRegister(m.prometheus.scheduleFinalCoverage)
	prometheus.MustRegister(m.prometheus.scheduleOverwrite)
}

func (m *MetricsCollectorSchedule) Reset() {
	m.prometheus.schedule.Reset()
	m.prometheus.scheduleLayer.Reset()
	m.prometheus.scheduleLayerEntry.Reset()
	m.prometheus.scheduleLayerCoverage.Reset()
	m.prometheus.scheduleFinalEntry.Reset()
	m.prometheus.scheduleFinalCoverage.Reset()
	m.prometheus.scheduleOverwrite.Reset()
}

func (m *MetricsCollectorSchedule) Collect(ctx context.Context, callback chan<- func()) {
	var wg sync.WaitGroup

	listOpts := pagerduty.ListSchedulesOptions{}
	listOpts.Limit = PagerdutyListLimit
	listOpts.Offset = 0

	scheduleMetricList := MetricCollectorList{}

	for {
		daemonLogger.Verbosef("fetch schedules (offset: %v, limit:%v)", listOpts.Offset, listOpts.Limit)

		list, err := PagerDutyClient.ListSchedules(listOpts)
		m.CollectorReference.PrometheusAPICounter().WithLabelValues("ListSchedules").Inc()

		if err != nil {
			panic(err)
		}

		for _, schedule := range list.Schedules {
			scheduleMetricList.AddInfo(prometheus.Labels{
				"scheduleID":       schedule.ID,
				"scheduleName":     schedule.Name,
				"scheduleTimeZone": schedule.TimeZone,
			})

			// get detail information about schedule
			wg.Add(1)
			go func(scheduleID string, callback chan<- func()) {
				defer wg.Done()
				m.collectScheduleInformation(scheduleID, callback)
				m.collectScheduleOverrides(scheduleID, callback)
			}(schedule.ID, callback)
		}

		listOpts.Offset += list.Limit
		if !list.More {
			break
		}
	}

	// set metrics
	callback <- func() {
		scheduleMetricList.GaugeSet(m.prometheus.schedule)
	}

	wg.Wait()
}

func (m *MetricsCollectorSchedule) collectScheduleInformation(scheduleID string, callback chan<- func()) {
	filterSince := time.Now().Add(-opts.ScrapeTime)
	filterUntil := time.Now().Add(opts.PagerDutyScheduleEntryTimeframe)

	listOpts := pagerduty.GetScheduleOptions{}
	listOpts.Limit = PagerdutyListLimit
	listOpts.Since = filterSince.Format(time.RFC3339)
	listOpts.Until = filterUntil.Format(time.RFC3339)
	listOpts.Offset = 0

	daemonLogger.Verbosef("fetch schedule information (schedule: %v, offset: %v, limit:%v)", scheduleID, listOpts.Offset, listOpts.Limit)

	schedule, err := PagerDutyClient.GetSchedule(scheduleID, listOpts)
	m.CollectorReference.PrometheusAPICounter().WithLabelValues("GetSchedule").Inc()

	if err != nil {
		panic(err)
	}

	scheduleLayerMetricList := MetricCollectorList{}
	scheduleLayerEntryMetricList := MetricCollectorList{}
	scheduleLayerCoverageMetricList := MetricCollectorList{}
	scheduleFinalEntryMetricList := MetricCollectorList{}
	scheduleFinalCoverageMetricList := MetricCollectorList{}

	for _, scheduleLayer := range schedule.ScheduleLayers {

		// schedule layer informations
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
				"time":            startTime.Format(opts.PagerDutyScheduleEntryTimeFormat),
				"type":            "startTime",
			}, startTime)

			// schedule item end
			scheduleLayerEntryMetricList.AddTime(prometheus.Labels{
				"scheduleID":      scheduleID,
				"scheduleLayerID": scheduleLayer.ID,
				"userID":          scheduleEntry.User.ID,
				"time":            endTime.Format(opts.PagerDutyScheduleEntryTimeFormat),
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
			"time":       startTime.Format(opts.PagerDutyScheduleEntryTimeFormat),
			"type":       "startTime",
		}, startTime)

		// schedule item end
		scheduleFinalEntryMetricList.AddTime(prometheus.Labels{
			"scheduleID": scheduleID,
			"userID":     scheduleEntry.User.ID,
			"time":       endTime.Format(opts.PagerDutyScheduleEntryTimeFormat),
			"type":       "endTime",
		}, endTime)
	}

	// final schedule coverage
	scheduleFinalCoverageMetricList.Add(prometheus.Labels{
		"scheduleID": scheduleID,
	}, schedule.FinalSchedule.RenderedCoveragePercentage)

	// set metrics
	callback <- func() {
		scheduleLayerMetricList.GaugeSet(m.prometheus.scheduleLayer)
		scheduleLayerCoverageMetricList.GaugeSet(m.prometheus.scheduleLayerCoverage)
		scheduleLayerEntryMetricList.GaugeSet(m.prometheus.scheduleLayerEntry)
		scheduleFinalEntryMetricList.GaugeSet(m.prometheus.scheduleFinalEntry)
		scheduleFinalCoverageMetricList.GaugeSet(m.prometheus.scheduleFinalCoverage)
	}
}

func (m *MetricsCollectorSchedule) collectScheduleOverrides(scheduleID string, callback chan<- func()) {
	filterSince := time.Now().Add(-opts.ScrapeTime)
	filterUntil := time.Now().Add(opts.PagerDutyScheduleOverrideTimeframe)

	listOpts := pagerduty.ListOverridesOptions{}
	listOpts.Limit = PagerdutyListLimit
	listOpts.Since = filterSince.Format(time.RFC3339)
	listOpts.Until = filterUntil.Format(time.RFC3339)
	listOpts.Offset = 0

	overrideMetricList := MetricCollectorList{}

	for {
		daemonLogger.Verbosef("fetch schedule overrides (schedule: %v, offset: %v, limit:%v)", scheduleID, listOpts.Offset, listOpts.Limit)

		list, err := PagerDutyClient.ListOverrides(scheduleID, listOpts)
		m.CollectorReference.PrometheusAPICounter().WithLabelValues("ListOverrides").Inc()

		if err != nil {
			panic(err)
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

		listOpts.Offset += list.Limit
		if !list.More {
			break
		}
	}

	// set metrics
	callback <- func() {
		overrideMetricList.GaugeSet(m.prometheus.scheduleOverwrite)
	}
}

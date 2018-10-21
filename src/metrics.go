package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"sync"
	"time"
)

var (
	prometheusApiCounter *prometheus.GaugeVec
	prometheusTeam *prometheus.GaugeVec
	prometheusUser *prometheus.GaugeVec
	prometheusService *prometheus.GaugeVec
	prometheusMaintenanceWindows *prometheus.GaugeVec
	prometheusMaintenanceWindowsStatus *prometheus.GaugeVec
	prometheusSchedule *prometheus.GaugeVec
	prometheusScheduleLayer *prometheus.GaugeVec
	prometheusScheduleLayerEntry *prometheus.GaugeVec
	prometheusScheduleLayerCoverage *prometheus.GaugeVec
	prometheusScheduleFinalEntry *prometheus.GaugeVec
	prometheusScheduleFinalCoverage *prometheus.GaugeVec
	prometheusScheduleOnCall *prometheus.GaugeVec
	prometheusScheduleOverwrite *prometheus.GaugeVec
	prometheusIncident *prometheus.GaugeVec
	prometheusIncidentStatus *prometheus.GaugeVec
)

type prometheusEntry struct {
	labels prometheus.Labels
	value float64
}

// Create and setup metrics and collection
func setupMetricsCollection() {
	prometheusApiCounter = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pagerduty_api_counter",
			Help: "PagerDuty api call counter",
		},
		[]string{"type"},
	)

	prometheusTeam = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pagerduty_team_info",
			Help: "PagerDuty team",
		},
		[]string{"teamID", "teamName", "teamUrl"},
	)

	prometheusUser = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pagerduty_user_info",
			Help: "PagerDuty user",
		},
		[]string{"userID", "userName", "userMail", "userAvatar", "userColor", "userJobTitle", "userRole", "userTimezone"},
	)

	prometheusService = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pagerduty_service_info",
			Help: "PagerDuty service",
		},
		[]string{"serviceID", "teamID", "serviceName", "serviceUrl"},
	)

	prometheusMaintenanceWindows = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pagerduty_maintenancewindow_info",
			Help: "PagerDuty MaintenanceWindow",
		},
		[]string{"windowID", "serviceID"},
	)

	prometheusMaintenanceWindowsStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pagerduty_maintenancewindow_status",
			Help: "PagerDuty MaintenanceWindow",
		},
		[]string{"windowID", "serviceID", "type"},
	)

	prometheusSchedule = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pagerduty_schedule_info",
			Help: "PagerDuty schedule",
		},
		[]string{"scheduleID", "scheduleName", "scheduleTimeZone"},
	)

	prometheusScheduleLayer = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pagerduty_schedule_layer_info",
			Help: "PagerDuty schedule layer informations",
		},
		[]string{"scheduleID", "scheduleLayerID", "scheduleLayerName"},
	)

	prometheusScheduleLayerEntry = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pagerduty_schedule_layer_entry",
			Help: "PagerDuty schedule layer entries",
		},
		[]string{"scheduleLayerID", "scheduleID", "userID", "time", "type"},
	)

	prometheusScheduleLayerCoverage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pagerduty_schedule_layer_coverage",
			Help: "PagerDuty schedule layer entry coverage",
		},
		[]string{"scheduleLayerID", "scheduleID"},
	)

	prometheusScheduleFinalEntry = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pagerduty_schedule_final_entry",
			Help: "PagerDuty schedule final entries",
		},
		[]string{"scheduleID", "userID", "time", "type"},
	)

	prometheusScheduleFinalCoverage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pagerduty_schedule_final_coverage",
			Help: "PagerDuty schedule final entry coverage",
		},
		[]string{"scheduleID"},
	)

	prometheusScheduleOnCall = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pagerduty_schedule_oncall",
			Help: "PagerDuty schedule oncall",
		},
		[]string{"scheduleID", "userID", "escalationLevel", "type"},
	)

	prometheusScheduleOverwrite = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pagerduty_schedule_override",
			Help: "PagerDuty schedule override",
		},
		[]string{"overrideID", "scheduleID", "userID", "type"},
	)

	prometheusIncident = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pagerduty_incident_info",
			Help: "PagerDuty oncall",
		},
		[]string{"incidentID", "serviceID", "incidentUrl", "incidentNumber", "title", "status", "urgency", "acknowledged", "assigned", "type", "time"},
	)

	prometheusIncidentStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pagerduty_incident_status",
			Help: "PagerDuty oncall",
		},
		[]string{"incidentID", "userID", "time", "type"},
	)

	prometheus.MustRegister(prometheusApiCounter)
	prometheus.MustRegister(prometheusTeam)
	prometheus.MustRegister(prometheusUser)
	prometheus.MustRegister(prometheusService)
	prometheus.MustRegister(prometheusMaintenanceWindows)
	prometheus.MustRegister(prometheusMaintenanceWindowsStatus)
	prometheus.MustRegister(prometheusSchedule)
	prometheus.MustRegister(prometheusScheduleLayer)
	prometheus.MustRegister(prometheusScheduleLayerEntry)
	prometheus.MustRegister(prometheusScheduleLayerCoverage)
	prometheus.MustRegister(prometheusScheduleFinalEntry)
	prometheus.MustRegister(prometheusScheduleFinalCoverage)
	prometheus.MustRegister(prometheusScheduleOnCall)
	prometheus.MustRegister(prometheusScheduleOverwrite)
	prometheus.MustRegister(prometheusIncident)
	prometheus.MustRegister(prometheusIncidentStatus)
}

// Start backgrounded metrics collection
func startMetricsCollection() {
	// general informations
	go func() {
		for {
			go func() {
				runMetricsCollectionGeneral()
			}()
			time.Sleep(opts.ScrapeTime)
		}
	}()

	// incidents informations
	go func() {
		for {
			go func() {
				runMetricsCollectionIncidents()
			}()
			time.Sleep(opts.ScrapeTimeIncidents)
		}
	}()
}

// Metrics run
func runMetricsCollectionGeneral() {
	var wg sync.WaitGroup

	callbackChannel := make(chan func())

	// Team info
	wg.Add(1)
	go func() {
		defer wg.Done()
		collectTeams(callbackChannel)
	}()

	// Team info
	wg.Add(1)
	go func() {
		defer wg.Done()
		collectUser(callbackChannel)
	}()

	// Service
	wg.Add(1)
	go func() {
		defer wg.Done()
		collectServices(callbackChannel)
	}()

	// MaintenanceWindows
	wg.Add(1)
	go func() {
		defer wg.Done()
		collectMaintenanceWindows(callbackChannel)
	}()

	// Schedules
	wg.Add(1)
	go func() {
		defer wg.Done()
		collectSchedules(callbackChannel)
	}()

	// Schedules OnCalls
	wg.Add(1)
	go func() {
		defer wg.Done()
		collectScheduleOnCalls(callbackChannel)
	}()

	go func() {
		var callbackList []func()
		for callback := range callbackChannel {
			callbackList = append(callbackList, callback)
		}

		prometheusTeam.Reset()
		prometheusUser.Reset()
		prometheusService.Reset()
		prometheusMaintenanceWindows.Reset()
		prometheusMaintenanceWindowsStatus.Reset()
		prometheusSchedule.Reset()
		prometheusScheduleLayer.Reset()
		prometheusScheduleLayerEntry.Reset()
		prometheusScheduleLayerCoverage.Reset()
		prometheusScheduleFinalEntry.Reset()
		prometheusScheduleFinalCoverage.Reset()
		prometheusScheduleOnCall.Reset()
		prometheusScheduleOverwrite.Reset()
		for _, callback := range callbackList {
			callback()
		}

		Logger.Messsage("run[general]: finished")
	}()

	// wait for all funcs
	wg.Wait()
	close(callbackChannel)
}

// Metrics run (incidents only)
func runMetricsCollectionIncidents() {
	var wg sync.WaitGroup

	callbackChannel := make(chan func())
	// Incidents
	wg.Add(1)
	go func() {
		defer wg.Done()
		collectIncidents(callbackChannel)
	}()

	go func() {
		var callbackList []func()
		for callback := range callbackChannel {
			callbackList = append(callbackList, callback)
		}

		prometheusIncident.Reset()
		prometheusIncidentStatus.Reset()
		for _, callback := range callbackList {
			callback()
		}

		Logger.Messsage("run[incidents]: finished")
	}()

	// wait for all funcs
	wg.Wait()
	close(callbackChannel)
}

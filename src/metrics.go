package main

import (
	"github.com/mblaschke/go-pagerduty"
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
		[]string{"userID", "userName", "userMail"},
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

func collectTeams(callback chan<- func()) {
	listOpts := pagerduty.ListTeamOptions{}
	listOpts.Limit = PAGERDUTY_LIST_LIMIT
	listOpts.Offset = 0

	teamList := []prometheusEntry{}
	
	for {
		Logger.Verbose(" - fetch teams (offset: %v, limit:%v)", listOpts.Offset, listOpts.Limit)

		list, err := PagerDutyClient.ListTeams(listOpts)
		prometheusApiCounter.WithLabelValues("ListTeams").Inc()
	
		if err != nil {
			panic(err)
		}
	
		for _, team := range list.Teams {
			row := prometheusEntry{
				labels: prometheus.Labels{
					"teamID": team.ID,
					"teamName": team.Name,
					"teamUrl": team.HTMLURL,
				},
				value: 1,
			}
			teamList = append(teamList, row)
		}

		listOpts.Offset += list.Limit
		if !list.More {
			break
		}
	}

	// set metrics
	callback <- func() {
		for _, row := range teamList {
			prometheusTeam.With(row.labels).Set(row.value)
		}
	}
}

func collectUser(callback chan<- func()) {
	listOpts := pagerduty.ListUsersOptions{}
	listOpts.Limit = PAGERDUTY_LIST_LIMIT
	listOpts.Offset = 0

	userList := []prometheusEntry{}
	
	for {
		Logger.Verbose(" - fetch users (offset: %v, limit:%v)", listOpts.Offset, listOpts.Limit)

		list, err := PagerDutyClient.ListUsers(listOpts)
		prometheusApiCounter.WithLabelValues("ListUsers").Inc()

		if err != nil {
			panic(err)
		}
	
		for _, user := range list.Users {
			row := prometheusEntry{
				labels: prometheus.Labels{
					"userID": user.ID,
					"userName": user.Name,
					"userMail": user.Email,
				},
				value: 1,
			}
			userList = append(userList, row)
		}

		listOpts.Offset += list.Limit
		if !list.More {
			break
		}
	}

	// set metrics
	callback <- func() {
		for _, row := range userList {
			prometheusUser.With(row.labels).Set(row.value)
		}
	}
}


func collectServices(callback chan<- func()) {
	listOpts := pagerduty.ListServiceOptions{}
	listOpts.Limit = PAGERDUTY_LIST_LIMIT
	listOpts.Offset = 0

	serviceList := []prometheusEntry{}

	for {
		Logger.Verbose(" - fetch services (offset: %v, limit:%v)", listOpts.Offset, listOpts.Limit)

		list, err := PagerDutyClient.ListServices(listOpts)
		prometheusApiCounter.WithLabelValues("ListServices").Inc()


		if err != nil {
			panic(err)
		}
	
		for _, service := range list.Services {
			for _, team := range service.Teams {
				row := prometheusEntry{
					labels: prometheus.Labels{
						"serviceID": service.ID,
						"teamID": team.ID,
						"serviceName": service.Name,
						"serviceUrl": service.HTMLURL,
					},
					value: 1,
				}
				serviceList = append(serviceList, row)
			}
		}

		listOpts.Offset += list.Limit
		if !list.More {
			break
		}
	}

	// set metrics
	callback <- func() {
		for _, row := range serviceList {
			prometheusService.With(row.labels).Set(row.value)
		}
	}
}


func collectMaintenanceWindows(callback chan<- func()) {
	listOpts := pagerduty.ListMaintenanceWindowsOptions{}
	listOpts.Limit = PAGERDUTY_LIST_LIMIT
	listOpts.Offset = 0

	maintenanceWindowList := []prometheusEntry{}
	maintenanceWindowStatusList := []prometheusEntry{}

	for {
		Logger.Verbose(" - fetch maintenance windows (offset: %v, limit:%v)", listOpts.Offset, listOpts.Limit)

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

func collectSchedules(callback chan<- func()) {
	listOpts := pagerduty.ListSchedulesOptions{}
	listOpts.Limit = PAGERDUTY_LIST_LIMIT
	listOpts.Offset = 0

	scheduleList := []prometheusEntry{}

	for {
		Logger.Verbose(" - fetch schedules (offset: %v, limit:%v)", listOpts.Offset, listOpts.Limit)

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
		Logger.Verbose(" - fetch schedule oncalls (offset: %v, limit:%v)", listOpts.Offset, listOpts.Limit)

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

	Logger.Verbose(" - fetch schedule information (schedule: %v, offset: %v, limit:%v)", scheduleId, listOpts.Offset, listOpts.Limit)

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
		Logger.Verbose(" - fetch schedule overrides (schedule: %v, offset: %v, limit:%v)", scheduleId, listOpts.Offset, listOpts.Limit)

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


func collectIncidents(callback chan<- func()) {
	listOpts := pagerduty.ListIncidentsOptions{}
	listOpts.Limit = PAGERDUTY_LIST_LIMIT
	listOpts.Statuses = []string{"triggered", "acknowledged"}
	listOpts.Offset = 0

	incidentList := []prometheusEntry{}
	incidentStatusList := []prometheusEntry{}

	for {
		Logger.Verbose(" - fetch incidents (offset: %v, limit:%v)", listOpts.Offset, listOpts.Limit)

		list, err := PagerDutyClient.ListIncidents(listOpts)
		prometheusApiCounter.WithLabelValues("ListIncidents").Inc()

		if err != nil {
			panic(err)
		}

		for _, incident := range list.Incidents {
			// info
			createdAt, _ := time.Parse(time.RFC3339, incident.CreatedAt)
			row := prometheusEntry{
				labels: prometheus.Labels{
					"incidentID": incident.ID,
					"serviceID": incident.Service.ID,
					"incidentUrl": incident.HTMLURL,
					"incidentNumber": uintToString(incident.IncidentNumber),
					"title": incident.Title,
					"status": incident.Status,
					"urgency": incident.Urgency,
					"acknowledged": boolToString(len(incident.Acknowledgements) >= 1),
					"assigned": boolToString(len(incident.Assignments) >= 1),
					"type": incident.Type,
					"time": createdAt.Format(opts.PagerDutyIncidentTimeFormat),
				},
				value: float64(createdAt.Unix()),
			}
			incidentList = append(incidentList, row)
			
			// acknowledgement
			for _, acknowledgement := range incident.Acknowledgements {
				createdAt, _ := time.Parse(time.RFC3339, acknowledgement.At)
				row := prometheusEntry{
					labels: prometheus.Labels{
						"incidentID": incident.ID,
						"userID": acknowledgement.Acknowledger.ID,
						"time": createdAt.Format(opts.PagerDutyIncidentTimeFormat),
						"type": "acknowledgement",
					},
					value: float64(createdAt.Unix()),
				}
				incidentStatusList = append(incidentStatusList, row)
			}
			
			// assignment
			for _, assignment := range incident.Assignments {
				createdAt, _ := time.Parse(time.RFC3339, assignment.At)
				row := prometheusEntry{
					labels: prometheus.Labels{
						"incidentID": incident.ID,
						"userID": assignment.Assignee.ID,
						"time": createdAt.Format(opts.PagerDutyIncidentTimeFormat),
						"type": "assignment",
					},
					value: float64(createdAt.Unix()),
				}
				incidentStatusList = append(incidentStatusList, row)
			}
		}

		listOpts.Offset += list.Limit
		if !list.More {
			break
		}
	}

	// set metrics
	callback <- func() {
		for _, row := range incidentList {
			prometheusIncident.With(row.labels).Set(row.value)
		}

		for _, row := range incidentStatusList {
			prometheusIncidentStatus.With(row.labels).Set(row.value)
		}
	}
}

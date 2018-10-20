package main

import (
	"sync"
	"time"
	"github.com/mblaschke/go-pagerduty"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	prometheusTeam *prometheus.GaugeVec
	prometheusUser *prometheus.GaugeVec
	prometheusService *prometheus.GaugeVec
	prometheusMaintenanceWindows *prometheus.GaugeVec
	prometheusMaintenanceWindowsStatus *prometheus.GaugeVec
	prometheusSchedule *prometheus.GaugeVec
	prometheusScheduleOnCall *prometheus.GaugeVec
	prometheusScheduleOverwrite *prometheus.GaugeVec
	prometheusIncident *prometheus.GaugeVec
)

// Create and setup metrics and collection
func setupMetricsCollection() {
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
		[]string{"scheduleID", "userID", "type"},
	)

	prometheusIncident = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pagerduty_incident_info",
			Help: "PagerDuty oncall",
		},
		[]string{"incidentID", "incidentUrl", "incidentNumber", "title", "status", "urgency", "acknowledgements", "assignments", "type"},
	)

	prometheus.MustRegister(prometheusTeam)
	prometheus.MustRegister(prometheusUser)
	prometheus.MustRegister(prometheusService)
	prometheus.MustRegister(prometheusMaintenanceWindows)
	prometheus.MustRegister(prometheusMaintenanceWindowsStatus)
	prometheus.MustRegister(prometheusSchedule)
	prometheus.MustRegister(prometheusScheduleOnCall)
	prometheus.MustRegister(prometheusScheduleOverwrite)
	prometheus.MustRegister(prometheusIncident)
}

// Start backgrounded metrics collection
func startMetricsCollection() {
	go func() {
		for {
			go func() {
				runMetricsCollection()
			}()
			time.Sleep(opts.ScrapeTime)
		}
	}()
}

// Metrics run
func runMetricsCollection() {
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

		prometheusTeam.Reset()
		prometheusUser.Reset()
		prometheusService.Reset()
		prometheusMaintenanceWindows.Reset()
		prometheusMaintenanceWindowsStatus.Reset()
		prometheusSchedule.Reset()
		prometheusScheduleOnCall.Reset()
		prometheusScheduleOverwrite.Reset()
		prometheusIncident.Reset()
		for _, callback := range callbackList {
			callback()
		}

		Logger.Messsage("run[queue]: finished")
	}()

	// wait for all funcs
	wg.Wait()
	close(callbackChannel)
}

func collectTeams(callback chan<- func()) {
	listOpts := pagerduty.ListTeamOptions{}
	listOpts.Limit = PAGERDUTY_LIST_LIMIT
	listOpts.Offset = 0
	
	for {
		Logger.Verbose(" - fetch teams (offset: %v, limit:%v)", listOpts.Offset, listOpts.Limit)

		list, err := PagerDutyClient.ListTeams(listOpts)
	
		if err != nil {
			panic(err)
		}
	
		for _, team := range list.Teams {
			infoLabels := prometheus.Labels{
				"teamID": team.ID,
				"teamName": team.Name,
				"teamUrl": team.HTMLURL,
			}
	
			callback <- func() {
				prometheusTeam.With(infoLabels).Set(1)
			}
		}

		listOpts.Offset += list.Limit
		if !list.More {
			break
		}
	}
}

func collectUser(callback chan<- func()) {
	listOpts := pagerduty.ListUsersOptions{}
	listOpts.Limit = PAGERDUTY_LIST_LIMIT
	listOpts.Offset = 0
	
	for {
		Logger.Verbose(" - fetch users (offset: %v, limit:%v)", listOpts.Offset, listOpts.Limit)

		list, err := PagerDutyClient.ListUsers(listOpts)
	
		if err != nil {
			panic(err)
		}
	
		for _, user := range list.Users {
			infoLabels := prometheus.Labels{
				"userID": user.ID,
				"userName": user.Name,
				"userMail": user.Email,
			}
	
			callback <- func() {
				prometheusUser.With(infoLabels).Set(1)
			}
		}

		listOpts.Offset += list.Limit
		if !list.More {
			break
		}
	}
}


func collectServices(callback chan<- func()) {
	listOpts := pagerduty.ListServiceOptions{}
	listOpts.Limit = PAGERDUTY_LIST_LIMIT
	listOpts.Offset = 0
	
	for {
		Logger.Verbose(" - fetch services (offset: %v, limit:%v)", listOpts.Offset, listOpts.Limit)

		list, err := PagerDutyClient.ListServices(listOpts)
	
		if err != nil {
			panic(err)
		}
	
		for _, service := range list.Services {
			for _, team := range service.Teams {
				infoLabels := prometheus.Labels{
					"serviceID": service.ID,
					"teamID": team.ID,
					"serviceName": service.Name,
					"serviceUrl": service.HTMLURL,
				}
	
				callback <- func() {
					prometheusService.With(infoLabels).Set(1)
				}
			}
		}

		listOpts.Offset += list.Limit
		if !list.More {
			break
		}
	}
}


func collectMaintenanceWindows(callback chan<- func()) {
	listOpts := pagerduty.ListMaintenanceWindowsOptions{}
	listOpts.Limit = PAGERDUTY_LIST_LIMIT
	listOpts.Offset = 0
	
	for {
		Logger.Verbose(" - fetch maintenance windows (offset: %v, limit:%v)", listOpts.Offset, listOpts.Limit)

		list, err := PagerDutyClient.ListMaintenanceWindows(listOpts)
	
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
				infoLabels := prometheus.Labels{
					"serviceID": service.ID,
					"windowID": maintWindow.ID,
				}
	
				statusStartTimeLabels := prometheus.Labels{
					"windowID": service.ID,
					"serviceID": service.ID,
					"type": "startTime",
				}
	
				statusEndTimeLabels := prometheus.Labels{
					"windowID": service.ID,
					"serviceID": service.ID,
					"type": "endTime",
				}
	
				callback <- func() {
					prometheusMaintenanceWindows.With(infoLabels).Set(1)
					prometheusMaintenanceWindowsStatus.With(statusStartTimeLabels).Set(float64(startTime.Unix()))
					prometheusMaintenanceWindowsStatus.With(statusEndTimeLabels).Set(float64(endTime.Unix()))
				}
			}
		}

		listOpts.Offset += list.Limit
		if !list.More {
			break
		}
	}
}

func collectSchedules(callback chan<- func()) {
	listOpts := pagerduty.ListSchedulesOptions{}
	listOpts.Limit = PAGERDUTY_LIST_LIMIT
	listOpts.Offset = 0
	
	for {
		Logger.Verbose(" - fetch schedules (offset: %v, limit:%v)", listOpts.Offset, listOpts.Limit)

		list, err := PagerDutyClient.ListSchedules(listOpts)
	
		if err != nil {
			panic(err)
		}
	
		for _, schedule := range list.Schedules {
			infoLabels := prometheus.Labels{
				"scheduleID": schedule.ID,
				"scheduleName": schedule.Name,
				"scheduleTimeZone": schedule.TimeZone,
			}
	
			callback <- func() {
				prometheusSchedule.With(infoLabels).Set(1)
			}

			collectScheduleOverrides(schedule.ID, callback)
		}

		listOpts.Offset += list.Limit
		if !list.More {
			break
		}
	}
}

func collectScheduleOnCalls(callback chan<- func()) {
	listOpts := pagerduty.ListOnCallOptions{}
	listOpts.Limit = PAGERDUTY_LIST_LIMIT
	listOpts.Earliest = true
	listOpts.Offset = 0
	
	for {
		Logger.Verbose(" - fetch schedule oncalls (offset: %v, limit:%v)", listOpts.Offset, listOpts.Limit)

		list, err := PagerDutyClient.ListOnCalls(listOpts)
	
		if err != nil {
			panic(err)
		}
	
		for _, oncall := range list.OnCalls {
			startTime, _ := time.Parse(time.RFC3339, oncall.Start)
			endTime, _ := time.Parse(time.RFC3339, oncall.End)
	
			startLabels := prometheus.Labels{
				"scheduleID": oncall.Schedule.ID,
				"userID": oncall.User.ID,
				"escalationLevel": uintToString(oncall.EscalationLevel),
				"type": "startTime",
			}
			startValue := float64(startTime.Unix())
	
			endLabels := prometheus.Labels{
				"scheduleID": oncall.Schedule.ID,
				"userID": oncall.User.ID,
				"escalationLevel": uintToString(oncall.EscalationLevel),
				"type": "endTime",
			}
			endValue := float64(endTime.Unix())
	
			if startValue < 0 {
				startValue = 1
			}
	
			if endValue < 0 {
				endValue = 1
			}
	
			callback <- func() {
				prometheusScheduleOnCall.With(startLabels).Set(startValue)
				prometheusScheduleOnCall.With(endLabels).Set(endValue)
			}
		}

		// loop
		listOpts.Offset += list.Limit
		if !list.More {
			break
		}
	}
}


func collectScheduleOverrides(scheduleId string, callback chan<- func()) {
	filterSince := time.Now().Add(-opts.ScrapeTime)
	filterUntil := time.Now().Add(opts.PagerScheduleOverrideDuration)

	listOpts := pagerduty.ListOverridesOptions{}
	listOpts.Limit = PAGERDUTY_LIST_LIMIT
	listOpts.Since = filterSince.Format(time.RFC3339)
	listOpts.Until = filterUntil.Format(time.RFC3339)
	listOpts.Offset = 0

	for {
		Logger.Verbose(" - fetch schedule overrides (schedule: %v, offset: %v, limit:%v)", scheduleId, listOpts.Offset, listOpts.Limit)

		list, err := PagerDutyClient.ListOverrides(scheduleId, listOpts)

		if err != nil {
			panic(err)
		}

		for _, override := range list.Overrides {
			startTime, _ := time.Parse(time.RFC3339, override.Start)
			endTime, _ := time.Parse(time.RFC3339, override.End)

			startLabels := prometheus.Labels{
				"scheduleID": scheduleId,
				"userID": override.User.ID,
				"type": "startTime",
			}
			startValue := float64(startTime.Unix())

			endLabels := prometheus.Labels{
				"scheduleID": scheduleId,
				"userID": override.User.ID,
				"type": "endTime",
			}
			endValue := float64(endTime.Unix())

			callback <- func() {
				prometheusScheduleOverwrite.With(startLabels).Set(startValue)
				prometheusScheduleOverwrite.With(endLabels).Set(endValue)
			}
		}

		listOpts.Offset += list.Limit
		if !list.More {
			break
		}
	}
}


func collectIncidents(callback chan<- func()) {
	listOpts := pagerduty.ListIncidentsOptions{}
	listOpts.Limit = PAGERDUTY_LIST_LIMIT
	listOpts.Statuses = []string{"triggered", "acknowledged"}
	listOpts.Offset = 0

	for {
		Logger.Verbose(" - fetch incidents (offset: %v, limit:%v)", listOpts.Offset, listOpts.Limit)

		list, err := PagerDutyClient.ListIncidents(listOpts)

		if err != nil {
			panic(err)
		}

		for _, incident := range list.Incidents {
			createdAt, _ := time.Parse(time.RFC3339, incident.CreatedAt)

			infoLabels := prometheus.Labels{
				"incidentID": incident.ID,
				"incidentUrl": incident.HTMLURL,
				"incidentNumber": uintToString(incident.IncidentNumber),
				"title": incident.Title,
				"status": incident.Status,
				"urgency": incident.Urgency,
				"acknowledgements": intToString(len(incident.Acknowledgements)),
				"assignments": intToString(len(incident.Assignments)),
				"type": incident.Type,
			}

			callback <- func() {
				prometheusIncident.With(infoLabels).Set(float64(createdAt.Unix()))
			}
		}

		listOpts.Offset += list.Limit
		if !list.More {
			break
		}
	}
}

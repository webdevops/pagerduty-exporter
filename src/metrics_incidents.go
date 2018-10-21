package main

import (
	"time"
	"github.com/mblaschke/go-pagerduty"
	"github.com/prometheus/client_golang/prometheus"
)

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

			// lastChange
			changedAt, _ := time.Parse(time.RFC3339, incident.LastStatusChangeAt)
			rowChange := prometheusEntry{
				labels: prometheus.Labels{
					"incidentID": incident.ID,
					"userID": incident.LastStatusChangeBy.ID,
					"time": changedAt.Format(opts.PagerDutyIncidentTimeFormat),
					"type": "lastChange",
				},
				value: float64(changedAt.Unix()),
			}
			incidentStatusList = append(incidentStatusList, rowChange)
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

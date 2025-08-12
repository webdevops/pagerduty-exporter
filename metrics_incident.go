package main

import (
	"time"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/webdevops/go-common/prometheus/collector"
)

type MetricsCollectorIncident struct {
	collector.Processor

	prometheus struct {
		incident       *prometheus.GaugeVec
		incidentStatus *prometheus.GaugeVec
		incidentMTTA   *prometheus.GaugeVec
		serviceMTTA    *prometheus.GaugeVec
		incidentMTTR   *prometheus.GaugeVec
		serviceMTTR    *prometheus.GaugeVec
	}

	teamListOpt []string
}

func (m *MetricsCollectorIncident) Setup(collector *collector.Collector) {
	m.Processor.Setup(collector)

	m.prometheus.incident = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pagerduty_incident_info",
			Help: "PagerDuty incident",
		},
		[]string{
			"incidentID",
			"serviceID",
			"incidentUrl",
			"incidentNumber",
			"title",
			"status",
			"urgency",
			"acknowledged",
			"assigned",
			"type",
			"time",
		},
	)
	m.Collector.RegisterMetricList("pagerduty_incident_info", m.prometheus.incident, true)

	m.prometheus.incidentStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pagerduty_incident_status",
			Help: "PagerDuty incident status",
		},
		[]string{
			"incidentID",
			"userID",
			"time",
			"type",
		},
	)
	m.Collector.RegisterMetricList("pagerduty_incident_status", m.prometheus.incidentStatus, true)

	m.prometheus.incidentMTTA = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pagerduty_incident_mtta_seconds",
			Help: "PagerDuty incident Mean Time To Acknowledgment in seconds",
		},
		[]string{
			"incidentID",
			"serviceID",
			"serviceName",
			"urgency",
			"acknowledgerID",
		},
	)
	m.Collector.RegisterMetricList("pagerduty_incident_mtta_seconds", m.prometheus.incidentMTTA, true)

	m.prometheus.serviceMTTA = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pagerduty_service_mtta_seconds",
			Help: "PagerDuty service-level Mean Time To Acknowledgment in seconds (rolling average)",
		},
		[]string{
			"serviceID",
			"serviceName",
			"urgency",
		},
	)
	m.Collector.RegisterMetricList("pagerduty_service_mtta_seconds", m.prometheus.serviceMTTA, true)

	m.prometheus.incidentMTTR = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pagerduty_incident_mttr_seconds",
			Help: "PagerDuty incident Mean Time To Resolution in seconds",
		},
		[]string{
			"incidentID",
			"serviceID",
			"serviceName",
			"urgency",
			"resolverID",
		},
	)
	m.Collector.RegisterMetricList("pagerduty_incident_mttr_seconds", m.prometheus.incidentMTTR, true)

	m.prometheus.serviceMTTR = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pagerduty_service_mttr_seconds",
			Help: "PagerDuty service-level Mean Time To Resolution in seconds (rolling average)",
		},
		[]string{
			"serviceID",
			"serviceName",
			"urgency",
		},
	)
	m.Collector.RegisterMetricList("pagerduty_service_mttr_seconds", m.prometheus.serviceMTTR, true)
}

func (m *MetricsCollectorIncident) Reset() {
}

func (m *MetricsCollectorIncident) Collect(callback chan<- func()) {
	listOpts := pagerduty.ListIncidentsOptions{}
	listOpts.Limit = PagerdutyListLimit
	listOpts.Statuses = Opts.PagerDuty.Incident.Statuses
	listOpts.Offset = 0
	listOpts.SortBy = "created_at:desc"

	if len(m.teamListOpt) > 0 {
		listOpts.TeamIDs = m.teamListOpt
	}

	// Ensure we also fetch resolved incidents for MTTR calculations
	// if not already included in the configured statuses
	includesResolved := false
	for _, status := range Opts.PagerDuty.Incident.Statuses {
		if status == "resolved" || status == "all" {
			includesResolved = true
			break
		}
	}

	var resolvedIncidents []pagerduty.Incident
	if !includesResolved {
		// Fetch resolved incidents separately for MTTR calculations
		resolvedOpts := pagerduty.ListIncidentsOptions{}
		resolvedOpts.Limit = PagerdutyListLimit
		resolvedOpts.Statuses = []string{"resolved"}
		resolvedOpts.Offset = 0
		resolvedOpts.SortBy = "created_at:desc"

		if len(m.teamListOpt) > 0 {
			resolvedOpts.TeamIDs = m.teamListOpt
		}

		m.Logger().Debugf("fetch resolved incidents for MTTR (offset: %v, limit:%v)", resolvedOpts.Offset, resolvedOpts.Limit)

		for {
			resolvedList, err := PagerDutyClient.ListIncidentsWithContext(m.Context(), resolvedOpts)
			PrometheusPagerDutyApiCounter.WithLabelValues("ListIncidents").Inc()

			if err != nil {
				m.Logger().Panic(err)
			}

			resolvedIncidents = append(resolvedIncidents, resolvedList.Incidents...)

			resolvedOpts.Offset += resolvedOpts.Limit
			if stopPagerdutyPaging(resolvedList.APIListObject) || resolvedOpts.Offset >= Opts.PagerDuty.Incident.Limit {
				break
			}
		}
	}

	incidentMetricList := m.Collector.GetMetricList("pagerduty_incident_info")
	incidentStatusMetricList := m.Collector.GetMetricList("pagerduty_incident_status")
	incidentMTTAMetricList := m.Collector.GetMetricList("pagerduty_incident_mtta_seconds")
	serviceMTTAMetricList := m.Collector.GetMetricList("pagerduty_service_mtta_seconds")
	incidentMTTRMetricList := m.Collector.GetMetricList("pagerduty_incident_mttr_seconds")
	serviceMTTRMetricList := m.Collector.GetMetricList("pagerduty_service_mttr_seconds")

	// Track MTTA and MTTR data per service for calculating averages
	serviceMTTAData := make(map[string][]float64) // key: serviceID_urgency, value: []mttaSeconds
	serviceMTTRData := make(map[string][]float64) // key: serviceID_urgency, value: []mttrSeconds

	for {
		m.Logger().Debugf("fetch incidents (offset: %v, limit:%v)", listOpts.Offset, listOpts.Limit)

		list, err := PagerDutyClient.ListIncidentsWithContext(m.Context(), listOpts)
		PrometheusPagerDutyApiCounter.WithLabelValues("ListIncidents").Inc()

		if err != nil {
			m.Logger().Panic(err)
		}

		for _, incident := range list.Incidents {
			// info
			createdAt, _ := time.Parse(time.RFC3339, incident.CreatedAt)

			incidentMetricList.AddTime(prometheus.Labels{
				"incidentID":     incident.ID,
				"serviceID":      incident.Service.ID,
				"incidentUrl":    incident.HTMLURL,
				"incidentNumber": uintToString(incident.IncidentNumber),
				"title":          incident.Title,
				"status":         incident.Status,
				"urgency":        incident.Urgency,
				"acknowledged":   boolToString(len(incident.Acknowledgements) >= 1),
				"assigned":       boolToString(len(incident.Assignments) >= 1),
				"type":           incident.Type,
				"time":           createdAt.Format(Opts.PagerDuty.Incident.TimeFormat),
			}, createdAt)

			// acknowledgement
			for _, acknowledgement := range incident.Acknowledgements {
				createdAt, _ := time.Parse(time.RFC3339, acknowledgement.At)
				incidentStatusMetricList.AddTime(prometheus.Labels{
					"incidentID": incident.ID,
					"userID":     acknowledgement.Acknowledger.ID,
					"time":       createdAt.Format(Opts.PagerDuty.Incident.TimeFormat),
					"type":       "acknowledgement",
				}, createdAt)
			}

			// assignment
			for _, assignment := range incident.Assignments {
				createdAt, _ := time.Parse(time.RFC3339, assignment.At)
				incidentStatusMetricList.AddTime(prometheus.Labels{
					"incidentID": incident.ID,
					"userID":     assignment.Assignee.ID,
					"time":       createdAt.Format(Opts.PagerDuty.Incident.TimeFormat),
					"type":       "assignment",
				}, createdAt)
			}

			// lastChange
			changedAt, _ := time.Parse(time.RFC3339, incident.LastStatusChangeAt)
			incidentStatusMetricList.AddTime(prometheus.Labels{
				"incidentID": incident.ID,
				"userID":     incident.LastStatusChangeBy.ID,
				"time":       changedAt.Format(Opts.PagerDuty.Incident.TimeFormat),
				"type":       "lastChange",
			}, changedAt)

			// Calculate MTTA (Mean Time To Acknowledgment) for acknowledged incidents
			if len(incident.Acknowledgements) > 0 {
				// Find the first acknowledgment
				var firstAck *pagerduty.Acknowledgement
				var firstAckTime time.Time

				for i := range incident.Acknowledgements {
					ackTime, err := time.Parse(time.RFC3339, incident.Acknowledgements[i].At)
					if err != nil {
						continue
					}

					if firstAck == nil || ackTime.Before(firstAckTime) {
						firstAck = &incident.Acknowledgements[i]
						firstAckTime = ackTime
					}
				}

				if firstAck != nil {
					// Get service name
					serviceName := m.getServiceName(incident.Service.ID)

					// Calculate MTTA in seconds
					mttaSeconds := firstAckTime.Sub(createdAt).Seconds()

					incidentMTTAMetricList.Add(prometheus.Labels{
						"incidentID":     incident.ID,
						"serviceID":      incident.Service.ID,
						"serviceName":    serviceName,
						"urgency":        incident.Urgency,
						"acknowledgerID": firstAck.Acknowledger.ID,
					}, mttaSeconds)

					// Track for service-level MTTA calculation
					serviceKey := incident.Service.ID + "_" + incident.Urgency
					serviceMTTAData[serviceKey] = append(serviceMTTAData[serviceKey], mttaSeconds)
				}
			}

			// Calculate MTTR (Mean Time To Resolution) for resolved incidents
			if incident.Status == "resolved" && incident.LastStatusChangeAt != "" {
				resolvedAt, err := time.Parse(time.RFC3339, incident.LastStatusChangeAt)
				if err == nil {
					// Get service name
					serviceName := m.getServiceName(incident.Service.ID)

					// Calculate MTTR in seconds
					mttrSeconds := resolvedAt.Sub(createdAt).Seconds()

					// Get resolver ID from LastStatusChangeBy
					resolverID := ""
					if incident.LastStatusChangeBy.ID != "" {
						resolverID = incident.LastStatusChangeBy.ID
					}

					incidentMTTRMetricList.Add(prometheus.Labels{
						"incidentID":  incident.ID,
						"serviceID":   incident.Service.ID,
						"serviceName": serviceName,
						"urgency":     incident.Urgency,
						"resolverID":  resolverID,
					}, mttrSeconds)

					// Track for service-level MTTR calculation
					serviceKey := incident.Service.ID + "_" + incident.Urgency
					serviceMTTRData[serviceKey] = append(serviceMTTRData[serviceKey], mttrSeconds)
				}
			}
		}

		listOpts.Offset += PagerdutyListLimit
		if stopPagerdutyPaging(list.APIListObject) || listOpts.Offset >= Opts.PagerDuty.Incident.Limit {
			break
		}
	}

	// Process resolved incidents for MTTR if they were fetched separately
	if len(resolvedIncidents) > 0 {
		m.Logger().Debugf("processing %d resolved incidents for MTTR", len(resolvedIncidents))
		for _, incident := range resolvedIncidents {
			createdAt, _ := time.Parse(time.RFC3339, incident.CreatedAt)

			// Calculate MTTR (Mean Time To Resolution) for resolved incidents
			if incident.Status == "resolved" && incident.LastStatusChangeAt != "" {
				resolvedAt, err := time.Parse(time.RFC3339, incident.LastStatusChangeAt)
				if err == nil {
					// Get service name
					serviceName := m.getServiceName(incident.Service.ID)

					// Calculate MTTR in seconds
					mttrSeconds := resolvedAt.Sub(createdAt).Seconds()

					// Get resolver ID from LastStatusChangeBy
					resolverID := ""
					if incident.LastStatusChangeBy.ID != "" {
						resolverID = incident.LastStatusChangeBy.ID
					}

					incidentMTTRMetricList.Add(prometheus.Labels{
						"incidentID":  incident.ID,
						"serviceID":   incident.Service.ID,
						"serviceName": serviceName,
						"urgency":     incident.Urgency,
						"resolverID":  resolverID,
					}, mttrSeconds)

					// Track for service-level MTTR calculation
					serviceKey := incident.Service.ID + "_" + incident.Urgency
					serviceMTTRData[serviceKey] = append(serviceMTTRData[serviceKey], mttrSeconds)
				}
			}
		}
	}

	// Calculate and set service-level MTTA averages
	serviceNames := make(map[string]string) // serviceID -> serviceName mapping
	for serviceKey, mttaValues := range serviceMTTAData {
		if len(mttaValues) == 0 {
			continue
		}

		// Parse serviceKey (format: serviceID_urgency)
		lastUnderscore := len(serviceKey) - 1
		for i := len(serviceKey) - 1; i >= 0; i-- {
			if serviceKey[i] == '_' {
				lastUnderscore = i
				break
			}
		}
		serviceID := serviceKey[:lastUnderscore]
		urgency := serviceKey[lastUnderscore+1:]

		// Get service name (cache it to avoid multiple API calls)
		serviceName, exists := serviceNames[serviceID]
		if !exists {
			serviceName = m.getServiceName(serviceID)
			serviceNames[serviceID] = serviceName
		}

		// Calculate average MTTA
		var total float64
		for _, mtta := range mttaValues {
			total += mtta
		}
		avgMTTA := total / float64(len(mttaValues))

		// Set service-level metric
		serviceMTTAMetricList.Add(prometheus.Labels{
			"serviceID":   serviceID,
			"serviceName": serviceName,
			"urgency":     urgency,
		}, avgMTTA)
	}

	// Calculate and set service-level MTTR averages
	for serviceKey, mttrValues := range serviceMTTRData {
		if len(mttrValues) == 0 {
			continue
		}

		// Parse serviceKey (format: serviceID_urgency)
		lastUnderscore := len(serviceKey) - 1
		for i := len(serviceKey) - 1; i >= 0; i-- {
			if serviceKey[i] == '_' {
				lastUnderscore = i
				break
			}
		}
		serviceID := serviceKey[:lastUnderscore]
		urgency := serviceKey[lastUnderscore+1:]

		// Get service name (cache it to avoid multiple API calls)
		serviceName, exists := serviceNames[serviceID]
		if !exists {
			serviceName = m.getServiceName(serviceID)
			serviceNames[serviceID] = serviceName
		}

		// Calculate average MTTR
		var total float64
		for _, mttr := range mttrValues {
			total += mttr
		}
		avgMTTR := total / float64(len(mttrValues))

		// Set service-level metric
		serviceMTTRMetricList.Add(prometheus.Labels{
			"serviceID":   serviceID,
			"serviceName": serviceName,
			"urgency":     urgency,
		}, avgMTTR)
	}
}

func (m *MetricsCollectorIncident) getServiceName(serviceID string) string {
	service, err := PagerDutyClient.GetServiceWithContext(m.Context(), serviceID, &pagerduty.GetServiceOptions{})
	PrometheusPagerDutyApiCounter.WithLabelValues("GetService").Inc()

	if err != nil {
		m.Logger().Debugf("Failed to get service name for %s: %v", serviceID, err)
		return ""
	}

	return service.Name
}

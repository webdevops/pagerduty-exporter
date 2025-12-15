package main

import (
	"strings"
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
			"priority",
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
			"priority",
		},
	)
	m.Collector.RegisterMetricList("pagerduty_service_mttr_seconds", m.prometheus.serviceMTTR, true)
}

func (m *MetricsCollectorIncident) Reset() {
}

func (m *MetricsCollectorIncident) Collect(callback chan<- func()) {
	listOpts := pagerduty.ListIncidentsOptions{
		Includes: []string{"acknowledgers"},
	}
	listOpts.Limit = PagerdutyListLimit
	listOpts.Statuses = Opts.PagerDuty.Incident.Statuses
	listOpts.Offset = 0
	listOpts.SortBy = "created_at:desc"

	if len(m.teamListOpt) > 0 {
		listOpts.TeamIDs = m.teamListOpt
	}

	// Check if resolved incidents are already included in configured statuses
	includesResolved := false
	for _, status := range Opts.PagerDuty.Incident.Statuses {
		if status == "resolved" || status == "all" {
			includesResolved = true
			break
		}
	}

	// Fetch resolved incidents separately for MTTR/MTTA if not already included
	var resolvedIncidents []pagerduty.Incident
	if !includesResolved {
		resolvedOpts := pagerduty.ListIncidentsOptions{
			Includes: []string{"acknowledgers"},
		}
		resolvedOpts.Limit = PagerdutyListLimit
		resolvedOpts.Statuses = []string{"resolved"}
		resolvedOpts.Offset = 0
		resolvedOpts.SortBy = "created_at:desc"

		if len(m.teamListOpt) > 0 {
			resolvedOpts.TeamIDs = m.teamListOpt
		}

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

	// Track MTTA/MTTR data per service for calculating averages
	serviceMTTAData := make(map[string][]float64) // key: serviceID_urgency
	serviceMTTRData := make(map[string][]float64) // key: serviceID|urgency|priority
	serviceNames := make(map[string]string)       // cache serviceID -> serviceName

	for {
		m.Logger().Debugf("fetch incidents (offset: %v, limit:%v)", listOpts.Offset, listOpts.Limit)

		list, err := PagerDutyClient.ListIncidentsWithContext(m.Context(), listOpts)
		PrometheusPagerDutyApiCounter.WithLabelValues("ListIncidents").Inc()

		if err != nil {
			m.Logger().Panic(err)
		}

		for _, incident := range list.Incidents {
			m.processIncident(incident, incidentMetricList, incidentStatusMetricList,
				incidentMTTAMetricList, incidentMTTRMetricList,
				serviceMTTAData, serviceMTTRData, serviceNames)
		}

		listOpts.Offset += PagerdutyListLimit
		if stopPagerdutyPaging(list.APIListObject) || listOpts.Offset >= Opts.PagerDuty.Incident.Limit {
			break
		}
	}

	// Process resolved incidents if fetched separately
	for _, incident := range resolvedIncidents {
		m.processIncident(incident, incidentMetricList, incidentStatusMetricList,
			incidentMTTAMetricList, incidentMTTRMetricList,
			serviceMTTAData, serviceMTTRData, serviceNames)
	}

	// Calculate and set service-level MTTA averages
	for serviceKey, mttaValues := range serviceMTTAData {
		if len(mttaValues) == 0 {
			continue
		}

		// Parse serviceKey (format: serviceID_urgency)
		lastUnderscore := strings.LastIndex(serviceKey, "_")
		if lastUnderscore == -1 {
			continue
		}
		serviceID := serviceKey[:lastUnderscore]
		urgency := serviceKey[lastUnderscore+1:]

		serviceName := m.getCachedServiceName(serviceID, serviceNames)

		var total float64
		for _, mtta := range mttaValues {
			total += mtta
		}
		avgMTTA := total / float64(len(mttaValues))

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

		// Parse serviceKey (format: serviceID|urgency|priority)
		parts := strings.Split(serviceKey, "|")
		if len(parts) != 3 {
			continue
		}
		serviceID, urgency, priority := parts[0], parts[1], parts[2]

		serviceName := m.getCachedServiceName(serviceID, serviceNames)

		var total float64
		for _, mttr := range mttrValues {
			total += mttr
		}
		avgMTTR := total / float64(len(mttrValues))

		serviceMTTRMetricList.Add(prometheus.Labels{
			"serviceID":   serviceID,
			"serviceName": serviceName,
			"urgency":     urgency,
			"priority":    priority,
		}, avgMTTR)
	}
}

func (m *MetricsCollectorIncident) processIncident(
	incident pagerduty.Incident,
	incidentMetricList, incidentStatusMetricList, incidentMTTAMetricList, incidentMTTRMetricList *collector.MetricList,
	serviceMTTAData, serviceMTTRData map[string][]float64,
	serviceNames map[string]string,
) {
	createdAt, _ := time.Parse(time.RFC3339, incident.CreatedAt)

	// Apply filtering for CDCE services, non-DT alerts, and P1/P2 priority
	shouldReport := m.shouldReportIncident(incident)

	if shouldReport {
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
	}

	// Track acknowledgements
	for _, acknowledgement := range incident.Acknowledgements {
		ackAt, _ := time.Parse(time.RFC3339, acknowledgement.At)
		incidentStatusMetricList.AddTime(prometheus.Labels{
			"incidentID": incident.ID,
			"userID":     acknowledgement.Acknowledger.ID,
			"time":       ackAt.Format(Opts.PagerDuty.Incident.TimeFormat),
			"type":       "acknowledgement",
		}, ackAt)
	}

	// Track assignments
	for _, assignment := range incident.Assignments {
		assignAt, _ := time.Parse(time.RFC3339, assignment.At)
		incidentStatusMetricList.AddTime(prometheus.Labels{
			"incidentID": incident.ID,
			"userID":     assignment.Assignee.ID,
			"time":       assignAt.Format(Opts.PagerDuty.Incident.TimeFormat),
			"type":       "assignment",
		}, assignAt)
	}

	// Track last status change
	changedAt, _ := time.Parse(time.RFC3339, incident.LastStatusChangeAt)
	incidentStatusMetricList.AddTime(prometheus.Labels{
		"incidentID": incident.ID,
		"userID":     incident.LastStatusChangeBy.ID,
		"time":       changedAt.Format(Opts.PagerDuty.Incident.TimeFormat),
		"type":       "lastChange",
	}, changedAt)

	// Calculate MTTA and MTTR only for filtered incidents
	if !shouldReport {
		return
	}

	serviceName := m.getCachedServiceName(incident.Service.ID, serviceNames)

	// Calculate MTTA
	acknowledgedAt, acknowledgerID := m.getFirstAcknowledgement(incident)
	if !acknowledgedAt.IsZero() {
		mttaSeconds := acknowledgedAt.Sub(createdAt).Seconds()

		incidentMTTAMetricList.Add(prometheus.Labels{
			"incidentID":     incident.ID,
			"serviceID":      incident.Service.ID,
			"serviceName":    serviceName,
			"urgency":        incident.Urgency,
			"acknowledgerID": acknowledgerID,
		}, mttaSeconds)

		serviceKey := incident.Service.ID + "_" + incident.Urgency
		serviceMTTAData[serviceKey] = append(serviceMTTAData[serviceKey], mttaSeconds)
	}

	// Calculate MTTR for resolved incidents
	if incident.Status == "resolved" && incident.LastStatusChangeAt != "" {
		resolvedAt, err := time.Parse(time.RFC3339, incident.LastStatusChangeAt)
		if err == nil {
			mttrSeconds := resolvedAt.Sub(createdAt).Seconds()

			resolverID := incident.LastStatusChangeBy.ID
			priority := ""
			if incident.Priority != nil {
				priority = incident.Priority.Name
			}

			incidentMTTRMetricList.Add(prometheus.Labels{
				"incidentID":  incident.ID,
				"serviceID":   incident.Service.ID,
				"serviceName": serviceName,
				"urgency":     incident.Urgency,
				"resolverID":  resolverID,
				"priority":    priority,
			}, mttrSeconds)

			serviceKey := incident.Service.ID + "|" + incident.Urgency + "|" + priority
			serviceMTTRData[serviceKey] = append(serviceMTTRData[serviceKey], mttrSeconds)
		}
	}
}

// getFirstAcknowledgement returns the earliest acknowledgement time and acknowledger ID
func (m *MetricsCollectorIncident) getFirstAcknowledgement(incident pagerduty.Incident) (time.Time, string) {
	var acknowledgedAt time.Time
	var acknowledgerID string

	// Try incident.Acknowledgements first
	for _, ack := range incident.Acknowledgements {
		ackTime, err := time.Parse(time.RFC3339, ack.At)
		if err != nil {
			continue
		}
		if acknowledgedAt.IsZero() || ackTime.Before(acknowledgedAt) {
			acknowledgedAt = ackTime
			acknowledgerID = ack.Acknowledger.ID
		}
	}

	// If empty, fetch from log entries (for resolved incidents)
	if acknowledgedAt.IsZero() {
		logEntries, err := PagerDutyClient.ListIncidentLogEntriesWithContext(m.Context(), incident.ID, pagerduty.ListIncidentLogEntriesOptions{
			Limit:      PagerdutyListLimit,
			IsOverview: true,
		})
		if err == nil {
			for _, entry := range logEntries.LogEntries {
				if strings.HasPrefix(entry.Type, "acknowledge_log_entry") {
					at, err := time.Parse(time.RFC3339, entry.CreatedAt)
					if err != nil {
						continue
					}
					if acknowledgedAt.IsZero() || at.Before(acknowledgedAt) {
						acknowledgedAt = at
						if entry.Agent.ID != "" {
							acknowledgerID = entry.Agent.ID
						}
					}
				}
			}
		}
	}

	return acknowledgedAt, acknowledgerID
}

func (m *MetricsCollectorIncident) getCachedServiceName(serviceID string, cache map[string]string) string {
	if name, exists := cache[serviceID]; exists {
		return name
	}
	name := m.getServiceName(serviceID)
	cache[serviceID] = name
	return name
}

func (m *MetricsCollectorIncident) getServiceName(serviceID string) string {
	service, err := PagerDutyClient.GetServiceWithContext(m.Context(), serviceID, &pagerduty.GetServiceOptions{})
	PrometheusPagerDutyApiCounter.WithLabelValues("GetService").Inc()

	if err != nil {
		return ""
	}
	return service.Name
}

// shouldReportIncident filters incidents:
// 1. Service summary must start with "CDCE"
// 2. Escalation policy summary must NOT end with "DT alerts"
// 3. Priority must be "P1" or "P2"
func (m *MetricsCollectorIncident) shouldReportIncident(incident pagerduty.Incident) bool {
	serviceSummary := m.getServiceSummary(incident)
	if !strings.HasPrefix(serviceSummary, "CDCE") {
		return false
	}

	escalationPolicySummary := m.getEscalationPolicySummary(incident)
	if escalationPolicySummary == "" || strings.HasSuffix(escalationPolicySummary, "DT alerts") {
		return false
	}

	if incident.Priority == nil || (incident.Priority.Name != "P1" && incident.Priority.Name != "P2") {
		return false
	}

	return true
}

func (m *MetricsCollectorIncident) getServiceSummary(incident pagerduty.Incident) string {
	if incident.Service.Summary != "" {
		return incident.Service.Summary
	}

	service, err := PagerDutyClient.GetServiceWithContext(m.Context(), incident.Service.ID, &pagerduty.GetServiceOptions{})
	PrometheusPagerDutyApiCounter.WithLabelValues("GetService").Inc()

	if err != nil {
		return ""
	}
	return service.Name
}

func (m *MetricsCollectorIncident) getEscalationPolicySummary(incident pagerduty.Incident) string {
	if incident.EscalationPolicy.Summary != "" {
		return incident.EscalationPolicy.Summary
	}

	ep, err := PagerDutyClient.GetEscalationPolicyWithContext(m.Context(), incident.EscalationPolicy.ID, &pagerduty.GetEscalationPolicyOptions{})
	PrometheusPagerDutyApiCounter.WithLabelValues("GetEscalationPolicy").Inc()

	if err != nil {
		return ""
	}
	return ep.Name
}

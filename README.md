PagerDuty Exporter
==================

[![license](https://img.shields.io/github/license/webdevops/pagerduty-exporter.svg)](https://github.com/webdevops/pagerduty-exporter/blob/master/LICENSE)
[![Docker](https://img.shields.io/badge/docker-webdevops%2Fpagerduty--exporter-blue.svg?longCache=true&style=flat&logo=docker)](https://hub.docker.com/r/webdevops/pagerduty-exporter/)
[![Docker Build Status](https://img.shields.io/docker/build/webdevops/pagerduty-exporter.svg)](https://hub.docker.com/r/webdevops/pagerduty-exporter/)

Prometheus exporter for PagerDuty informations (users, teams, schedules, oncalls, incidents...)

Configuration
-------------

Normally no configuration is needed but can be customized using environment variables.

| Environment variable              | DefaultValue                | Description                                                              |
|-----------------------------------|-----------------------------|--------------------------------------------------------------------------|
| `SCRAPE_TIME`                     | `15m`                       | Time (time.Duration) between API calls                                   |
| `SERVER_BIND`                     | `:8080`                     | IP/Port binding                                                          |
| `PAGERDUTY_AUTH_TOKEN`            | none                        | PagerDuty auth token                                                     |

Metrics
-------

| Metric                                | Description                                                                           |
|---------------------------------------|---------------------------------------------------------------------------------------|
| `pagerduty_team_info`                 | Team informations                                                                     |
| `pagerduty_user_info`                 | User informations                                                                     |
| `pagerduty_service_info`              | Service (per team) informations                                                       |
| `pagerduty_maintenancewindow_info`    | Maintenance window informations                                                       |
| `pagerduty_maintenancewindow_status`  | Maintenance window status (start and endtime)                                         |
| `pagerduty_schedule_info`             | Schedule informations                                                                 |
| `pagerduty_schedule_oncall`           | Schedule oncall informations                                                          |
| `pagerduty_incident_info`             | Incident informations                                                                     |

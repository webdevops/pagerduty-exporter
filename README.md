PagerDuty Exporter
==================

[![license](https://img.shields.io/github/license/webdevops/pagerduty-exporter.svg)](https://github.com/webdevops/pagerduty-exporter/blob/master/LICENSE)
[![Docker](https://img.shields.io/badge/docker-webdevops%2Fpagerduty--exporter-blue.svg?longCache=true&style=flat&logo=docker)](https://hub.docker.com/r/webdevops/pagerduty-exporter/)
[![Docker Build Status](https://img.shields.io/docker/build/webdevops/pagerduty-exporter.svg)](https://hub.docker.com/r/webdevops/pagerduty-exporter/)

Prometheus exporter for PagerDuty informations (users, teams, schedules, oncalls, incidents...)

Configuration
-------------

Normally no configuration is needed but can be customized using environment variables.

| Environment variable                   | DefaultValue                | Description                                                               |
|-----------------------------------------|-----------------------------|--------------------------------------------------------------------------|
| `SCRAPE_TIME`                           | `5m`                        | Time (time.Duration) for general informations                            |
| `SCRAPE_TIME_INCIDENTS`                 | `1m`                        | Time (time.Duration) for incidents                                       |
| `SERVER_BIND`                           | `:8080`                     | IP/Port binding                                                          |
| `PAGERDUTY_AUTH_TOKEN`                  | none                        | PagerDuty auth token                                                     |
| `PAGERDUTY_SCHEDULE_OVERRIDE_TIMEFRAME` | `48h`                       | PagerDuty schedule override list timeframe                               |
| `PAGERDUTY_SCHEDULE_ENTRY_TIMEFRAME`    | `72h`                       | PagerDuty schedule rendered list timeframe                               |
| `PAGERDUTY_SCHEDULE_ENTRY_TIMEFORMAT`   | `Mon, 02 Jan 15:04 MST`     | PagerDuty schedule entry timeformat (label)                              |

Metrics
-------

| Metric                                | Description                                                                           |
|---------------------------------------|---------------------------------------------------------------------------------------|
| `pagerduty_api_counter`               | PagerDuty api call counter                                                            |
| `pagerduty_team_info`                 | Team informations                                                                     |
| `pagerduty_user_info`                 | User informations                                                                     |
| `pagerduty_service_info`              | Service (per team) informations                                                       |
| `pagerduty_maintenancewindow_info`    | Maintenance window informations                                                       |
| `pagerduty_maintenancewindow_status`  | Maintenance window status (start and endtime)                                         |
| `pagerduty_schedule_info`             | Schedule informations                                                                 |
| `pagerduty_schedule_entry`            | Schedule rendered schedule entries                                                    |
| `pagerduty_schedule_coverage`         | Schedule rendered schedule coverage                                                   |
| `pagerduty_schedule_oncall`           | Schedule oncall informations                                                          |
| `pagerduty_schedule_override`         | Schedule override informations                                                        |
| `pagerduty_incident_info`             | Incident informations                                                                 |

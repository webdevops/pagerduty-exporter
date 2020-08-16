PagerDuty Exporter
==================

[![license](https://img.shields.io/github/license/webdevops/pagerduty-exporter.svg)](https://github.com/webdevops/pagerduty-exporter/blob/master/LICENSE)
[![Docker](https://img.shields.io/docker/cloud/automated/webdevops/pagerduty-exporter)](https://hub.docker.com/r/webdevops/pagerduty-exporter/)
[![Docker Build Status](https://img.shields.io/docker/cloud/build/webdevops/pagerduty-exporter)](https://hub.docker.com/r/webdevops/pagerduty-exporter/)

Prometheus exporter for PagerDuty informations (users, teams, schedules, oncalls, incidents...)

Configuration
-------------

```
Usage:
  pagerduty-exporter [OPTIONS]

Application Options:
      --debug                                 debug mode [$DEBUG]
  -v, --verbose                               verbose mode [$VERBOSE]
      --log.json                              Switch log output to json format [$LOG_JSON]
      --pagerduty.authtoken=                  PagerDuty auth token [$PAGERDUTY_AUTH_TOKEN]
      --pagerduty.schedule.override-duration= PagerDuty timeframe for fetching schedule overrides (time.Duration)
                                              (default: 48h) [$PAGERDUTY_SCHEDULE_OVERRIDE_TIMEFRAME]
      --pagerduty.schedule.entry-timeframe=   PagerDuty timeframe for fetching schedule entries (time.Duration)
                                              (default: 72h) [$PAGERDUTY_SCHEDULE_ENTRY_TIMEFRAME]
      --pagerduty.schedule.entry-timeformat=  PagerDuty schedule entry time format (label) (default: Mon, 02 Jan 15:04
                                              MST) [$PAGERDUTY_SCHEDULE_ENTRY_TIMEFORMAT]
      --pagerduty.incident.timeformat=        PagerDuty incident time format (label) (default: Mon, 02 Jan 15:04 MST)
                                              [$PAGERDUTY_INCIDENT_TIMEFORMAT]
      --pagerduty.disable-teams               Set to true to disable checking PagerDuty teams (for plans that don't
                                              include it) [$PAGERDUTY_DISABLE_TEAMS]
      --pagerduty.team-filter=                Passes team ID as a list option when applicable. [$PAGERDUTY_TEAM_FILTER]
      --pagerduty.max-connections=            Maximum numbers of TCP connections to PagerDuty API (concurrency)
                                              (default: 4) [$PAGERDUTY_MAX_CONNECTIONS]
      --bind=                                 Server address (default: :8080) [$SERVER_BIND]
      --scrape.time=                          Scrape time (time.duration) (default: 5m) [$SCRAPE_TIME]
      --scrape.time.live=                     Scrape time incidents and oncalls (time.duration) (default: 1m)
                                              [$SCRAPE_TIME_LIVE]

Help Options:
  -h, --help                                  Show this help message
```

Metrics
-------

| Metric                                | Scraper            | Description                                                                           |
|---------------------------------------|--------------------|---------------------------------------------------------------------------------------|
| `pagerduty_stats`                     | Collector          | Collector stats                                                                       |
| `pagerduty_api_counter`               | Collector          | PagerDuty api call counter                                                            |
| `pagerduty_team_info`                 | Team               | Team informations                                                                     |
| `pagerduty_user_info`                 | User               | User informations                                                                     |
| `pagerduty_service_info`              | Service            | Service (per team) informations                                                       |
| `pagerduty_maintenancewindow_info`    | MaintanaceWindows  | Maintenance window informations                                                       |
| `pagerduty_maintenancewindow_status`  | Maintenance window | status (start and endtime)                                         |
| `pagerduty_schedule_info`             | Schedule           | Schedule informations                                                                 |
| `pagerduty_schedule_layer_info`       | Schedule           | Schedule layer informations                                                           |
| `pagerduty_schedule_layer_entry`      | Schedule           | Schedule layer schedule entries                                                       |
| `pagerduty_schedule_layer_coverage`   | Schedule           | Schedule layer schedule coverage                                                      |
| `pagerduty_schedule_final_entry`      | Schedule           | Schedule final (rendered) schedule entries                                            |
| `pagerduty_schedule_final_coverage`   | Schedule           | Schedule final (rendered) schedule coverage                                           |
| `pagerduty_schedule_override`         | Schedule           | Schedule override informations                                                        |
| `pagerduty_schedule_oncall`           | Oncall             | Schedule oncall informations                                                          |
| `pagerduty_incident_info`             | Incident           | Incident informations                                                                 |
| `pagerduty_incident_status`           | Incident           | Incident status informations (acknowledgement, assignment)                            |

Prometheus queries
------------------

Current oncall person
```
pagerduty_schedule_oncall{scheduleID="$SCHEDULEID",type="startTime"}
* on (userID) group_left(userName) (pagerduty_user_info)
```

Next shift
```
bottomk(1,
  min by (userName, time) (
    pagerduty_schedule_final_entry{scheduleID="$SCHEDULEID",type="startTime"}
    * on (userID) group_left(userName) (pagerduty_user_info) 
  ) - time() > 0
)
```

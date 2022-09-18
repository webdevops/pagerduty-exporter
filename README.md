# PagerDuty Exporter

[![license](https://img.shields.io/github/license/webdevops/pagerduty-exporter.svg)](https://github.com/webdevops/pagerduty-exporter/blob/master/LICENSE)
[![DockerHub](https://img.shields.io/badge/DockerHub-webdevops%2Fpagerduty--exporter-blue)](https://hub.docker.com/r/webdevops/pagerduty-exporter/)
[![Quay.io](https://img.shields.io/badge/Quay.io-webdevops%2Fpagerduty--exporter-blue)](https://quay.io/repository/webdevops/pagerduty-exporter)

Prometheus exporter for PagerDuty information (users, teams, schedules, oncalls, incidents...)

## Configuration

```
Usage:
  pagerduty-exporter [OPTIONS]

Application Options:
      --log.debug                                                       debug mode [$LOG_DEBUG]
      --log.trace                                                       trace mode [$LOG_TRACE]
      --log.json                                                        Switch log output to json format [$LOG_JSON]
      --pagerduty.authtoken=                                            PagerDuty auth token [$PAGERDUTY_AUTH_TOKEN]
      --pagerduty.max-connections=                                      Maximum numbers of TCP connections to PagerDuty API
                                                                        (concurrency) (default: 4) [$PAGERDUTY_MAX_CONNECTIONS]
      --pagerduty.schedule.override-duration=                           PagerDuty timeframe for fetching schedule overrides
                                                                        (time.Duration) (default: 48h)
                                                                        [$PAGERDUTY_SCHEDULE_OVERRIDE_TIMEFRAME]
      --pagerduty.schedule.entry-timeframe=                             PagerDuty timeframe for fetching schedule entries
                                                                        (time.Duration) (default: 72h)
                                                                        [$PAGERDUTY_SCHEDULE_ENTRY_TIMEFRAME]
      --pagerduty.schedule.entry-timeformat=                            PagerDuty schedule entry time format (label) (default:
                                                                        Mon, 02 Jan 15:04 MST)
                                                                        [$PAGERDUTY_SCHEDULE_ENTRY_TIMEFORMAT]
      --pagerduty.incident.status=[triggered|acknowledged|resolved|all] PagerDuty incident status filter (eg. 'triggered',
                                                                        'acknowledged', 'resolved' or 'all') (default:
                                                                        triggered, acknowledged) [$PAGERDUTY_INCIDENT_STATUS]
      --pagerduty.incident.timeformat=                                  PagerDuty incident time format (label) (default: Mon, 02
                                                                        Jan 15:04 MST) [$PAGERDUTY_INCIDENT_TIMEFORMAT]
      --pagerduty.incident.limit=                                       PagerDuty incident limit count (default: 5000)
                                                                        [$PAGERDUTY_INCIDENT_LIMIT]
      --pagerduty.disable-teams                                         Set to true to disable checking PagerDuty teams (for
                                                                        plans that don't include it) [$PAGERDUTY_DISABLE_TEAMS]
      --pagerduty.team-filter=                                          Passes team ID as a list option when applicable.
                                                                        [$PAGERDUTY_TEAM_FILTER]
      --pagerduty.summary.since=                                        Timeframe which data should be fetched for summary
                                                                        metrics (time.Duration) (default: 730h)
                                                                        [$PAGERDUTY_SUMMARY_SINCE]
      --server.bind=                                                    Server address (default: :8080) [$SERVER_BIND]
      --server.timeout.read=                                            Server read timeout (default: 5s) [$SERVER_TIMEOUT_READ]
      --server.timeout.write=                                           Server write timeout (default: 10s)
                                                                        [$SERVER_TIMEOUT_WRITE]
      --scrape.time=                                                    Scrape time (time.duration) (default: 5m) [$SCRAPE_TIME]
      --scrape.time.maintenancewindow=                                  Scrape time for maintenance window metrics
                                                                        (time.duration; default is SCRAPE_TIME)
                                                                        [$SCRAPE_TIME_MAINTENANCEWINDOW]
      --scrape.time.schedule=                                           Scrape time for schedule metrics (time.duration; default
                                                                        is SCRAPE_TIME) [$SCRAPE_TIME_SCHEDULE]
      --scrape.time.service=                                            Scrape time for service metrics (time.duration; default
                                                                        is SCRAPE_TIME) [$SCRAPE_TIME_SERVICE]
      --scrape.time.team=                                               Scrape time for team metrics (time.duration; default is
                                                                        SCRAPE_TIME) [$SCRAPE_TIME_TEAM]
      --scrape.time.user=                                               Scrape time for user metrics (time.duration; default is
                                                                        SCRAPE_TIME) [$SCRAPE_TIME_USER]
      --scrape.time.summary=                                            Scrape time for general summary metrics (time.duration)
                                                                        (default: 15m) [$SCRAPE_TIME_SUMMARY]
      --scrape.time.live=                                               Scrape time incidents and oncalls (time.duration)
                                                                        (default: 1m) [$SCRAPE_TIME_LIVE]

Help Options:
  -h, --help                                                            Show this help message
```

`--pagerduty.authtoken=` is a required option. Please refer to the [documentation](https://support.pagerduty.com/docs/generating-api-keys)
on how to generate a token.

## Installing and Running the Exporter

### Go

You can get the exporter via the following command:

```
go get github.com/webdevops/pagerduty-exporter
```

From here on you will be able to run the exporter as described  [configuration](#Configuration) section.


### Container
A containerized version is available via `docker pull webdevops/pagerduty-exporter`
Alternatively you can build the image yourself locally:

```
git clone git@github.com:webdevops/pagerduty-exporter.git && cd pagerduty-exporter
docker build -t webdevops/pagerduty-exporter:latest .
```

You are now able to run you exporter locally in a container with the following command:
```
docker run --rm -ti -p 8080:8080 webdevops/pagerduty-exporter:latest --pagerduty.authtoken=YourGeneratedToken
```

This will run the container locally, mapping container port 8080 to local port 8080, allowing you to scrape the exporter on `127.0.0.1:8080/metrics`


## Metrics

| Metric                                          | Scraper            | Description                                                                                                          |
|-------------------------------------------------|--------------------|----------------------------------------------------------------------------------------------------------------------|
| `pagerduty_stats`                               | Collector          | Collector stats                                                                                                      |
| `pagerduty_api_counter`                         | Collector          | PagerDuty api call counter                                                                                           |
| `pagerduty_team_info`                           | Team               | Team informations                                                                                                    |
| `pagerduty_user_info`                           | User               | User informations                                                                                                    |
| `pagerduty_service_info`                        | Service            | Service (per team) informations                                                                                      |
| `pagerduty_maintenancewindow_info`              | MaintanaceWindows  | Maintenance window informations                                                                                      |
| `pagerduty_maintenancewindow_status`            | Maintenance window | status (start and endtime)                                                                                           |
| `pagerduty_schedule_info`                       | Schedule           | Schedule informations                                                                                                |
| `pagerduty_schedule_layer_info`                 | Schedule           | Schedule layer informations                                                                                          |
| `pagerduty_schedule_layer_entry`                | Schedule           | Schedule layer schedule entries                                                                                      |
| `pagerduty_schedule_layer_coverage`             | Schedule           | Schedule layer schedule coverage                                                                                     |
| `pagerduty_schedule_final_entry`                | Schedule           | Schedule final (rendered) schedule entries                                                                           |
| `pagerduty_schedule_final_coverage`             | Schedule           | Schedule final (rendered) schedule coverage                                                                          |
| `pagerduty_schedule_override`                   | Schedule           | Schedule override informations                                                                                       |
| `pagerduty_schedule_oncall`                     | Oncall             | Schedule oncall informations                                                                                         |
| `pagerduty_incident_info`                       | Incident           | Incident informations                                                                                                |
| `pagerduty_incident_status`                     | Incident           | Incident status informations (acknowledgement, assignment)                                                           |
| `pagerduty_summary_incident_count`              | Summary            | Count of incidents splitted by status, service, urgency and priority                                                 |
| `pagerduty_summary_incident_resolve_duration`   | Summary            | Histogram (buckets) for resolve duration splitted by service, urgency and priority                                   |
| `pagerduty_summary_incident_statuschange_count` | Summary            | Counter for new or changed status (eg triggered -> acknowledged) incidents splitted by service, urgency and priority |

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

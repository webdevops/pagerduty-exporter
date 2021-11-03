package config

import (
	"encoding/json"
	log "github.com/sirupsen/logrus"
	"time"
)

type (
	Opts struct {
		// logger
		Logger struct {
			Debug   bool `           long:"debug"        env:"DEBUG"    description:"debug mode"`
			Verbose bool `short:"v"  long:"verbose"      env:"VERBOSE"  description:"verbose mode"`
			LogJson bool `           long:"log.json"     env:"LOG_JSON" description:"Switch log output to json format"`
		}

		// PagerDuty settings
		PagerDuty struct {
			AuthToken      string `long:"pagerduty.authtoken"                      env:"PAGERDUTY_AUTH_TOKEN"                         description:"PagerDuty auth token" required:"true" json:"-"`
			MaxConnections int    `long:"pagerduty.max-connections"                env:"PAGERDUTY_MAX_CONNECTIONS"                    description:"Maximum numbers of TCP connections to PagerDuty API (concurrency)" default:"4"`

			Schedule struct {
				OverrideTimeframe time.Duration `long:"pagerduty.schedule.override-duration"     env:"PAGERDUTY_SCHEDULE_OVERRIDE_TIMEFRAME"        description:"PagerDuty timeframe for fetching schedule overrides (time.Duration)" default:"48h"`
				EntryTimeframe    time.Duration `long:"pagerduty.schedule.entry-timeframe"       env:"PAGERDUTY_SCHEDULE_ENTRY_TIMEFRAME"           description:"PagerDuty timeframe for fetching schedule entries (time.Duration)" default:"72h"`
				EntryTimeFormat   string        `long:"pagerduty.schedule.entry-timeformat"      env:"PAGERDUTY_SCHEDULE_ENTRY_TIMEFORMAT"          description:"PagerDuty schedule entry time format (label)" default:"Mon, 02 Jan 15:04 MST"`
			}

			Incident struct {
				Statuses   []string `long:"pagerduty.incident.status"                env:"PAGERDUTY_INCIDENT_STATUS" env-delim:";"      description:"PagerDuty incident status filter (eg. triggered, acknowledged, resolved)" default:"triggered" default:"acknowledged"` // nolint:staticcheck
				TimeFormat string   `long:"pagerduty.incident.timeformat"            env:"PAGERDUTY_INCIDENT_TIMEFORMAT"                description:"PagerDuty incident time format (label)" default:"Mon, 02 Jan 15:04 MST"`
			}

			Teams struct {
				Disable bool     `long:"pagerduty.disable-teams"                  env:"PAGERDUTY_DISABLE_TEAMS"                      description:"Set to true to disable checking PagerDuty teams (for plans that don't include it)"                `
				Filter  []string `long:"pagerduty.team-filter" env-delim:","      env:"PAGERDUTY_TEAM_FILTER"                        description:"Passes team ID as a list option when applicable."`
			}

			Summary struct {
				Since time.Duration `long:"pagerduty.summary.since"     env:"PAGERDUTY_SUMMARY_SINCE"        description:"Timeframe which data should be fetched for summary metrics (time.Duration)" default:"730h"`
			}
		}

		// general options
		ServerBind string `long:"bind"              env:"SERVER_BIND"   description:"Server address"     default:":8080"`

		ScrapeTime struct {
			General time.Duration `long:"scrape.time"          env:"SCRAPE_TIME"            description:"Scrape time (time.duration)"                              default:"5m"`
			Summary time.Duration `long:"scrape.time.summary"  env:"SCRAPE_TIME_SUMMARY"    description:"Scrape time for general summary metrics (time.duration)"  default:"15m"`
			Live    time.Duration `long:"scrape.time.live"     env:"SCRAPE_TIME_LIVE"       description:"Scrape time incidents and oncalls (time.duration)"        default:"1m"`
		}
	}
)

func (o *Opts) GetJson() []byte {
	jsonBytes, err := json.Marshal(o)
	if err != nil {
		log.Panic(err)
	}
	return jsonBytes
}

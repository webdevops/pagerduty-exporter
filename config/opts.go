package config

import (
	"encoding/json"
	"time"
)

type (
	Opts struct {
		// logger
		Logger struct {
			Debug       bool `long:"log.debug"    env:"LOG_DEBUG"  description:"debug mode"`
			Development bool `long:"log.devel"    env:"LOG_DEVEL"  description:"development mode"`
			Json        bool `long:"log.json"     env:"LOG_JSON"   description:"Switch log output to json format"`
		}


		// PagerDuty settings
		PagerDuty struct {
			AuthToken      string `long:"pagerduty.authtoken"                      env:"PAGERDUTY_AUTH_TOKEN"                         description:"PagerDuty auth token" json:"-"`
			AuthTokenFile  string `long:"pagerduty.authtokenfile"                  env:"PAGERDUTY_AUTH_TOKEN_FILE"                    description:"PagerDuty auth token as path to file"`
			MaxConnections int    `long:"pagerduty.max-connections"                env:"PAGERDUTY_MAX_CONNECTIONS"                    description:"Maximum numbers of TCP connections to PagerDuty API (concurrency)" default:"4"`

			Schedule struct {
				OverrideTimeframe time.Duration `long:"pagerduty.schedule.override-duration"     env:"PAGERDUTY_SCHEDULE_OVERRIDE_TIMEFRAME"        description:"PagerDuty timeframe for fetching schedule overrides (time.Duration)" default:"48h"`
				EntryTimeframe    time.Duration `long:"pagerduty.schedule.entry-timeframe"       env:"PAGERDUTY_SCHEDULE_ENTRY_TIMEFRAME"           description:"PagerDuty timeframe for fetching schedule entries (time.Duration)" default:"72h"`
				EntryTimeFormat   string        `long:"pagerduty.schedule.entry-timeformat"      env:"PAGERDUTY_SCHEDULE_ENTRY_TIMEFORMAT"          description:"PagerDuty schedule entry time format (label)" default:"Mon, 02 Jan 15:04 MST"`
			}

			Incident struct {
				Statuses   []string `long:"pagerduty.incident.status"                env:"PAGERDUTY_INCIDENT_STATUS" env-delim:";"      description:"PagerDuty incident status filter (eg. 'triggered', 'acknowledged', 'resolved' or 'all')" default:"triggered" default:"acknowledged" choice:"triggered"  choice:"acknowledged"  choice:"resolved"  choice:"all"` // nolint:staticcheck
				TimeFormat string   `long:"pagerduty.incident.timeformat"            env:"PAGERDUTY_INCIDENT_TIMEFORMAT"                description:"PagerDuty incident time format (label)" default:"Mon, 02 Jan 15:04 MST"`
				Limit      uint     `long:"pagerduty.incident.limit"                 env:"PAGERDUTY_INCIDENT_LIMIT"                     description:"PagerDuty incident limit count"         default:"5000"`
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
		Server struct {
			// general options
			Bind         string        `long:"server.bind"              env:"SERVER_BIND"           description:"Server address"        default:":8080"`
			ReadTimeout  time.Duration `long:"server.timeout.read"      env:"SERVER_TIMEOUT_READ"   description:"Server read timeout"   default:"5s"`
			WriteTimeout time.Duration `long:"server.timeout.write"     env:"SERVER_TIMEOUT_WRITE"  description:"Server write timeout"  default:"10s"`
		}

		// caching
		Cache struct {
			Path string `long:"cache.path" env:"CACHE_PATH" description:"Cache path (to folder, file://path... or azblob://storageaccount.blob.core.windows.net/containername or k8scm://{namespace}/{configmap}})"`
		}

		ScrapeTime struct {
			General           time.Duration  `long:"scrape.time"          env:"SCRAPE_TIME"            description:"Scrape time (time.duration)"                              default:"5m"`
			MaintenanceWindow *time.Duration `long:"scrape.time.maintenancewindow"  env:"SCRAPE_TIME_MAINTENANCEWINDOW"    description:"Scrape time for maintenance window metrics (time.duration; default is SCRAPE_TIME)"`
			Schedule          *time.Duration `long:"scrape.time.schedule"  env:"SCRAPE_TIME_SCHEDULE"    description:"Scrape time for schedule metrics (time.duration; default is SCRAPE_TIME)"`
			Service           *time.Duration `long:"scrape.time.service"  env:"SCRAPE_TIME_SERVICE"    description:"Scrape time for service metrics (time.duration; default is SCRAPE_TIME)"`
			Team              *time.Duration `long:"scrape.time.team"  env:"SCRAPE_TIME_TEAM"    description:"Scrape time for team metrics (time.duration; default is SCRAPE_TIME)"`
			User              *time.Duration `long:"scrape.time.user"  env:"SCRAPE_TIME_USER"    description:"Scrape time for user metrics (time.duration; default is SCRAPE_TIME)"`
			Summary           time.Duration  `long:"scrape.time.summary"  env:"SCRAPE_TIME_SUMMARY"    description:"Scrape time for general summary metrics (time.duration)"  default:"15m"`
			Live              time.Duration  `long:"scrape.time.live"     env:"SCRAPE_TIME_LIVE"       description:"Scrape time incidents and oncalls (time.duration)"        default:"1m"`
		}
	}
)

func (o *Opts) GetCachePath(path string) (ret *string) {
	if o.Cache.Path != "" {
		tmp := o.Cache.Path + "/" + path
		ret = &tmp
	}

	return
}

func (o *Opts) GetJson() []byte {
	jsonBytes, err := json.Marshal(o)
	if err != nil {
		panic(err)
	}
	return jsonBytes
}

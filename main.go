package main

import (
	"fmt"
	"github.com/PagerDuty/go-pagerduty"
	"github.com/jessevdk/go-flags"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"log"
	"net"
	"net/http"
	"os"
	"runtime"
	"time"
)

const (
	author = "webdevops.io"

	// PagerdutyListLimit limits the amount of items returned from an API query
	PagerdutyListLimit = 100

	// Number of failed fetches in a row before stopping the exporter
	CollectorErrorThreshold = 5
)

var (
	argparser            *flags.Parser
	verbose              bool
	daemonLogger         *DaemonLogger
	PagerDutyClient      *pagerduty.Client
	collectorGeneralList map[string]*CollectorGeneral

	// Git version information
	gitCommit = "<unknown>"
	gitTag    = "<unknown>"
)

var opts struct {
	// general settings
	Verbose []bool `long:"verbose" short:"v"        env:"VERBOSE"                description:"Verbose mode"`

	// server settings
	ServerBind     string        `long:"bind"               env:"SERVER_BIND"            description:"Server address"                                     default:":8080"`
	ScrapeTime     time.Duration `long:"scrape.time"        env:"SCRAPE_TIME"            description:"Scrape time (time.duration)"                        default:"5m"`
	ScrapeTimeLive time.Duration `long:"scrape.time.live"   env:"SCRAPE_TIME_LIVE"       description:"Scrape time incidents and oncalls (time.duration)"  default:"1m"`

	// PagerDuty settings
	PagerDutyAuthToken                 string        `long:"pagerduty.authtoken"                      env:"PAGERDUTY_AUTH_TOKEN"                         description:"PagerDuty auth token" required:"true"`
	PagerDutyScheduleOverrideTimeframe time.Duration `long:"pagerduty.schedule.override-duration"     env:"PAGERDUTY_SCHEDULE_OVERRIDE_TIMEFRAME"        description:"PagerDuty timeframe for fetching schedule overrides (time.Duration)" default:"48h"`
	PagerDutyScheduleEntryTimeframe    time.Duration `long:"pagerduty.schedule.entry-timeframe"       env:"PAGERDUTY_SCHEDULE_ENTRY_TIMEFRAME"           description:"PagerDuty timeframe for fetching schedule entries (time.Duration)" default:"72h"`
	PagerDutyScheduleEntryTimeFormat   string        `long:"pagerduty.schedule.entry-timeformat"      env:"PAGERDUTY_SCHEDULE_ENTRY_TIMEFORMAT"          description:"PagerDuty schedule entry time format (label)" default:"Mon, 02 Jan 15:04 MST"`
	PagerDutyIncidentTimeFormat        string        `long:"pagerduty.incident.timeformat"            env:"PAGERDUTY_INCIDENT_TIMEFORMAT"                description:"PagerDuty incident time format (label)" default:"Mon, 02 Jan 15:04 MST"`
	PagerDutyDisableTeams              bool          `long:"pagerduty.disable-teams"                  env:"PAGERDUTY_DISABLE_TEAMS"                      description:"Set to true to disable checking PagerDuty teams (for plans that don't include it)"                `
	PagerDutyTeamFilter                []string      `long:"pagerduty.team-filter" env-delim:","      env:"PAGERDUTY_TEAM_FILTER"                        description:"Passes team ID as a list option when applicable."`
	PagerDutyMaxConnections            int           `long:"pagerduty.max-connections"                env:"PAGERDUTY_MAX_CONNECTIONS"                    description:"Maximum numbers of TCP connections to PagerDuty API (concurrency)" default:"4"`
}

func main() {
	initArgparser()

	// set verbosity
	verbose = len(opts.Verbose) >= 1

	// Init logger
	daemonLogger = NewDaemonLogger(log.Lshortfile, verbose)
	defer daemonLogger.Close()

	daemonLogger.Infof("Init Pagerduty exporter v%s (%s; by %v)", gitTag, gitCommit, author)

	daemonLogger.Infof("Init PagerDuty client")
	initPagerDuty()

	daemonLogger.Infof("Starting metrics collection")
	daemonLogger.Infof("  scape time: %v", opts.ScrapeTime)
	daemonLogger.Infof("  scape time live: %v", opts.ScrapeTimeLive)
	initMetricCollector()

	daemonLogger.Infof("Starting http server on %s", opts.ServerBind)
	startHTTPServer()
}

// init argparser and parse/validate arguments
func initArgparser() {
	argparser = flags.NewParser(&opts, flags.Default)
	_, err := argparser.Parse()

	// check if there is an parse error
	if err != nil {
		if flagsErr, ok := err.(*flags.Error); ok && flagsErr.Type == flags.ErrHelp {
			os.Exit(0)
		} else {
			fmt.Println(err)
			fmt.Println()
			argparser.WriteHelp(os.Stdout)
			os.Exit(1)
		}
	}
}

// Init and build PagerDuty client
func initPagerDuty() {
	PagerDutyClient = pagerduty.NewClient(opts.PagerDutyAuthToken)
	PagerDutyClient.HTTPClient = &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxConnsPerHost:       opts.PagerDutyMaxConnections,
			MaxIdleConns:          opts.PagerDutyMaxConnections,
			IdleConnTimeout:       60 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			MaxIdleConnsPerHost:   runtime.GOMAXPROCS(0) + 1,
		},
	}
}

func initMetricCollector() {
	var collectorName string
	collectorGeneralList = map[string]*CollectorGeneral{}

	if !opts.PagerDutyDisableTeams {
		collectorName = "Team"
		if opts.ScrapeTime.Seconds() > 0 {
			collectorGeneralList[collectorName] = NewCollectorGeneral(collectorName, &MetricsCollectorTeam{})
			collectorGeneralList[collectorName].Run(opts.ScrapeTime)
		} else {
			daemonLogger.Infof("collector[%s]: disabled", collectorName)
		}
	}

	collectorName = "User"
	if opts.ScrapeTime.Seconds() > 0 {
		collectorGeneralList[collectorName] = NewCollectorGeneral(collectorName, &MetricsCollectorUser{teamListOpt: opts.PagerDutyTeamFilter})
		collectorGeneralList[collectorName].Run(opts.ScrapeTime)
	} else {
		daemonLogger.Infof("collector[%s]: disabled", collectorName)
	}

	collectorName = "Service"
	if opts.ScrapeTime.Seconds() > 0 {
		collectorGeneralList[collectorName] = NewCollectorGeneral(collectorName, &MetricsCollectorService{teamListOpt: opts.PagerDutyTeamFilter})
		collectorGeneralList[collectorName].Run(opts.ScrapeTime)
	} else {
		daemonLogger.Infof("collector[%s]: disabled", collectorName)
	}

	collectorName = "Schedule"
	if opts.ScrapeTime.Seconds() > 0 {
		collectorGeneralList[collectorName] = NewCollectorGeneral(collectorName, &MetricsCollectorSchedule{})
		collectorGeneralList[collectorName].Run(opts.ScrapeTime)
	} else {
		daemonLogger.Infof("collector[%s]: disabled", collectorName)
	}

	collectorName = "MaintenanceWindow"
	if opts.ScrapeTime.Seconds() > 0 {
		collectorGeneralList[collectorName] = NewCollectorGeneral(collectorName, &MetricsCollectorMaintenanceWindow{teamListOpt: opts.PagerDutyTeamFilter})
		collectorGeneralList[collectorName].Run(opts.ScrapeTime)
	} else {
		daemonLogger.Infof("collector[%s]: disabled", collectorName)
	}

	collectorName = "OnCall"
	if opts.ScrapeTimeLive.Seconds() > 0 {
		collectorGeneralList[collectorName] = NewCollectorGeneral(collectorName, &MetricsCollectorOncall{})
		collectorGeneralList[collectorName].Run(opts.ScrapeTimeLive)
	} else {
		daemonLogger.Infof("collector[%s]: disabled", collectorName)
	}

	collectorName = "Incident"
	if opts.ScrapeTimeLive.Seconds() > 0 {
		collectorGeneralList[collectorName] = NewCollectorGeneral(collectorName, &MetricsCollectorIncident{teamListOpt: opts.PagerDutyTeamFilter})
		collectorGeneralList[collectorName].Run(opts.ScrapeTimeLive)
	} else {
		daemonLogger.Infof("collector[%s]: disabled", collectorName)
	}

	collectorName = "Collector"
	collectorGeneralList[collectorName] = NewCollectorGeneral(collectorName, &MetricsCollectorCollector{})
	collectorGeneralList[collectorName].Run(time.Duration(10 * time.Second))
	collectorGeneralList[collectorName].SetIsHidden(true)
}

// start and handle prometheus handler
func startHTTPServer() {
	http.Handle("/metrics", promhttp.Handler())
	daemonLogger.Fatal(http.ListenAndServe(opts.ServerBind, nil))
}

package main

import (
	"os"
	"fmt"
	"time"
	"net/http"
	"github.com/jessevdk/go-flags"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/mblaschke/go-pagerduty"
)

const (
	Author  = "webdevops.io"
	Version = "0.5.2"
	PAGERDUTY_LIST_LIMIT = 100
)

var (
	argparser          *flags.Parser
	args               []string
	Logger             *DaemonLogger
	ErrorLogger        *DaemonLogger
	PagerDutyClient    *pagerduty.Client
)

var opts struct {
	// general settings
	Verbose     []bool `                long:"verbose" short:"v"        env:"VERBOSE"                description:"Verbose mode"`

	// server settings
	ServerBind  string `                long:"bind"                     env:"SERVER_BIND"            description:"Server address"                               default:":8080"`
	ScrapeTime  time.Duration `         long:"scrape-time"              env:"SCRAPE_TIME"            description:"Scrape time (time.duration)"                  default:"5m"`
	ScrapeTimeIncidents  time.Duration `long:"scrape-time-incidents"    env:"SCRAPE_TIME_INCIDENTS"  description:"Scrape time incidents (time.duration)"        default:"1m"`

	// PagerDuty settings
	PagerDutyAuthToken string `long:"pagerduty-auth-token"                                        env:"PAGERDUTY_AUTH_TOKEN"                         description:"PagerDuty auth token" required:"true"`
	PagerDutyScheduleOverrideTimeframe time.Duration `long:"pagerduty-schedule-override-duration" env:"PAGERDUTY_SCHEDULE_OVERRIDE_TIMEFRAME"        description:"PagerDuty timeframe for fetching schedule overrides (time.Duration)" default:"48h"`
	PagerDutyScheduleEntryTimeframe time.Duration `long:"pagerduty-schedule-entry-timeframe"      env:"PAGERDUTY_SCHEDULE_ENTRY_TIMEFRAME"           description:"PagerDuty timeframe for fetching schedule entries (time.Duration)" default:"72h"`
	PagerDutyScheduleEntryTimeFormat string `long:"pagerduty-schedule-entry-timeformat"           env:"PAGERDUTY_SCHEDULE_ENTRY_TIMEFORMAT"          description:"PagerDuty schedule entry time format (label)" default:"Mon, 02 Jan 15:04 MST"`
	PagerDutyIncidentTimeFormat string `long:"pagerduty-incident-timeformat"                      env:"PAGERDUTY_INCIDENT_TIMEFORMAT"                description:"PagerDuty incident time format (label)" default:"Mon, 02 Jan 15:04 MST"`
}

func main() {
	initArgparser()

	Logger = CreateDaemonLogger(0)
	ErrorLogger = CreateDaemonErrorLogger(0)

	// set verbosity
	Verbose = len(opts.Verbose) >= 1

	Logger.Messsage("Init Pagerduty exporter v%s (written by %v)", Version, Author)

	Logger.Messsage("Init PagerDuty client")
	initPagerDuty()

	Logger.Messsage("Starting metrics collection")
	Logger.Messsage("  scape time: %v", opts.ScrapeTime)
	setupMetricsCollection()
	startMetricsCollection()

	Logger.Messsage("Starting http server on %s", opts.ServerBind)
	startHttpServer()
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
}

// start and handle prometheus handler
func startHttpServer() {
	http.Handle("/metrics", promhttp.Handler())
	ErrorLogger.Fatal(http.ListenAndServe(opts.ServerBind, nil))
}

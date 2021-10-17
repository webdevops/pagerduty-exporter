package main

import (
	"fmt"
	"github.com/PagerDuty/go-pagerduty"
	"github.com/jessevdk/go-flags"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"github.com/webdevops/pagerduty-exporter/config"
	"net"
	"net/http"
	"os"
	"path"
	"runtime"
	"strings"
	"time"
)

const (
	author = "webdevops.io"

	// PagerdutyListLimit limits the amount of items returned from an API query
	PagerdutyListLimit = 100

	// CollectorErrorThreshold Number of failed fetches in a row before stopping the exporter
	CollectorErrorThreshold = 5
)

var (
	argparser *flags.Parser
	opts      config.Opts

	PagerDutyClient      *pagerduty.Client
	collectorGeneralList map[string]*CollectorGeneral

	// Git version information
	gitCommit = "<unknown>"
	gitTag    = "<unknown>"
)

func main() {
	initArgparser()

	log.Infof("starting pagerduty-exporter v%s (%s; %s; by %v)", gitTag, gitCommit, runtime.Version(), author)
	log.Info(string(opts.GetJson()))

	log.Infof("init PagerDuty client")
	initPagerDuty()

	log.Infof("starting metrics collection")
	initMetricCollector()

	log.Infof("starting http server on %s", opts.ServerBind)
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
			fmt.Println()
			argparser.WriteHelp(os.Stdout)
			os.Exit(1)
		}
	}

	// verbose level
	if opts.Logger.Verbose {
		log.SetLevel(log.DebugLevel)
	}

	// debug level
	if opts.Logger.Debug {
		log.SetReportCaller(true)
		log.SetLevel(log.TraceLevel)
		log.SetFormatter(&log.TextFormatter{
			CallerPrettyfier: func(f *runtime.Frame) (string, string) {
				s := strings.Split(f.Function, ".")
				funcName := s[len(s)-1]
				return funcName, fmt.Sprintf("%s:%d", path.Base(f.File), f.Line)
			},
		})
	}

	// json log format
	if opts.Logger.LogJson {
		log.SetReportCaller(true)
		log.SetFormatter(&log.JSONFormatter{
			DisableTimestamp: true,
			CallerPrettyfier: func(f *runtime.Frame) (string, string) {
				s := strings.Split(f.Function, ".")
				funcName := s[len(s)-1]
				return funcName, fmt.Sprintf("%s:%d", path.Base(f.File), f.Line)
			},
		})
	}
}

// Init and build PagerDuty client
func initPagerDuty() {
	PagerDutyClient = pagerduty.NewClient(opts.PagerDuty.AuthToken)
	PagerDutyClient.HTTPClient = &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxConnsPerHost:       opts.PagerDuty.MaxConnections,
			MaxIdleConns:          opts.PagerDuty.MaxConnections,
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

	if !opts.PagerDuty.DisableTeams {
		collectorName = "Team"
		if opts.ScrapeTime.Seconds() > 0 {
			collectorGeneralList[collectorName] = NewCollectorGeneral(collectorName, &MetricsCollectorTeam{})
			collectorGeneralList[collectorName].Run(opts.ScrapeTime)
		} else {
			log.WithField("collector", collectorName).Infof("collector disabled")
		}
	}

	collectorName = "User"
	if opts.ScrapeTime.Seconds() > 0 {
		collectorGeneralList[collectorName] = NewCollectorGeneral(collectorName, &MetricsCollectorUser{teamListOpt: opts.PagerDuty.TeamFilter})
		collectorGeneralList[collectorName].Run(opts.ScrapeTime)
	} else {
		log.WithField("collector", collectorName).Infof("collector disabled")
	}

	collectorName = "Service"
	if opts.ScrapeTime.Seconds() > 0 {
		collectorGeneralList[collectorName] = NewCollectorGeneral(collectorName, &MetricsCollectorService{teamListOpt: opts.PagerDuty.TeamFilter})
		collectorGeneralList[collectorName].Run(opts.ScrapeTime)
	} else {
		log.WithField("collector", collectorName).Infof("collector disabled")

	}

	collectorName = "Schedule"
	if opts.ScrapeTime.Seconds() > 0 {
		collectorGeneralList[collectorName] = NewCollectorGeneral(collectorName, &MetricsCollectorSchedule{})
		collectorGeneralList[collectorName].Run(opts.ScrapeTime)
	} else {
		log.WithField("collector", collectorName).Infof("collector disabled")
	}

	collectorName = "MaintenanceWindow"
	if opts.ScrapeTime.Seconds() > 0 {
		collectorGeneralList[collectorName] = NewCollectorGeneral(collectorName, &MetricsCollectorMaintenanceWindow{teamListOpt: opts.PagerDuty.TeamFilter})
		collectorGeneralList[collectorName].Run(opts.ScrapeTime)
	} else {
		log.WithField("collector", collectorName).Infof("collector disabled")
	}

	collectorName = "OnCall"
	if opts.ScrapeTimeLive.Seconds() > 0 {
		collectorGeneralList[collectorName] = NewCollectorGeneral(collectorName, &MetricsCollectorOncall{})
		collectorGeneralList[collectorName].Run(opts.ScrapeTimeLive)
	} else {
		log.WithField("collector", collectorName).Infof("collector disabled")
	}

	collectorName = "Incident"
	if opts.ScrapeTimeLive.Seconds() > 0 {
		collectorGeneralList[collectorName] = NewCollectorGeneral(collectorName, &MetricsCollectorIncident{teamListOpt: opts.PagerDuty.TeamFilter})
		collectorGeneralList[collectorName].Run(opts.ScrapeTimeLive)
	} else {
		log.WithField("collector", collectorName).Infof("collector disabled")
	}

	collectorName = "Summary"
	if opts.ScrapeTimeSummary.Seconds() > 0 {
		collectorGeneralList[collectorName] = NewCollectorGeneral(collectorName, &MetricsCollectorSummary{teamListOpt: opts.PagerDuty.TeamFilter})
		collectorGeneralList[collectorName].Run(opts.ScrapeTimeSummary)
	} else {
		log.WithField("collector", collectorName).Infof("collector disabled")
	}

	collectorName = "Collector"
	collectorGeneralList[collectorName] = NewCollectorGeneral(collectorName, &MetricsCollectorCollector{})
	collectorGeneralList[collectorName].Run(time.Duration(10 * time.Second))
	collectorGeneralList[collectorName].SetIsHidden(true)
}

// start and handle prometheus handler
func startHTTPServer() {
	// healthz
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if _, err := fmt.Fprint(w, "Ok"); err != nil {
			log.Error(err)
		}
	})

	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(opts.ServerBind, nil))
}

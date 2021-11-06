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
	"net/url"
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

	if len(opts.PagerDuty.Incident.Statuses) == 1 {
		if strings.ToLower(opts.PagerDuty.Incident.Statuses[0]) == "all" {
			opts.PagerDuty.Incident.Statuses = []string{
				"triggered",
				"acknowledged",
				"resolved",
			}
		}
	}
}

// Init and build PagerDuty client
func initPagerDuty() {
	PagerDutyClient = pagerduty.NewClient(opts.PagerDuty.AuthToken)

	httpClientTransportProxy := http.ProxyFromEnvironment
	if opts.Logger.Debug {
		httpClientTransportProxy = func(req *http.Request) (*url.URL, error) {
			log.Debugf("send request to %v", req.URL.String())
			return http.ProxyFromEnvironment(req)
		}
	}

	PagerDutyClient.HTTPClient = &http.Client{
		Transport: &http.Transport{
			Proxy: httpClientTransportProxy,
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

	if !opts.PagerDuty.Teams.Disable {
		collectorName = "Team"
		if opts.ScrapeTime.General.Seconds() > 0 {
			collectorGeneralList[collectorName] = NewCollectorGeneral(collectorName, &MetricsCollectorTeam{})
			collectorGeneralList[collectorName].Run(opts.ScrapeTime.General)
		} else {
			log.WithField("collector", collectorName).Infof("collector disabled")
		}
	}

	collectorName = "User"
	if opts.ScrapeTime.General.Seconds() > 0 {
		collectorGeneralList[collectorName] = NewCollectorGeneral(collectorName, &MetricsCollectorUser{teamListOpt: opts.PagerDuty.Teams.Filter})
		collectorGeneralList[collectorName].Run(opts.ScrapeTime.General)
	} else {
		log.WithField("collector", collectorName).Infof("collector disabled")
	}

	collectorName = "Service"
	if opts.ScrapeTime.General.Seconds() > 0 {
		collectorGeneralList[collectorName] = NewCollectorGeneral(collectorName, &MetricsCollectorService{teamListOpt: opts.PagerDuty.Teams.Filter})
		collectorGeneralList[collectorName].Run(opts.ScrapeTime.General)
	} else {
		log.WithField("collector", collectorName).Infof("collector disabled")

	}

	collectorName = "Schedule"
	if opts.ScrapeTime.General.Seconds() > 0 {
		collectorGeneralList[collectorName] = NewCollectorGeneral(collectorName, &MetricsCollectorSchedule{})
		collectorGeneralList[collectorName].Run(opts.ScrapeTime.General)
	} else {
		log.WithField("collector", collectorName).Infof("collector disabled")
	}

	collectorName = "MaintenanceWindow"
	if opts.ScrapeTime.General.Seconds() > 0 {
		collectorGeneralList[collectorName] = NewCollectorGeneral(collectorName, &MetricsCollectorMaintenanceWindow{teamListOpt: opts.PagerDuty.Teams.Filter})
		collectorGeneralList[collectorName].Run(opts.ScrapeTime.General)
	} else {
		log.WithField("collector", collectorName).Infof("collector disabled")
	}

	collectorName = "OnCall"
	if opts.ScrapeTime.Live.Seconds() > 0 {
		collectorGeneralList[collectorName] = NewCollectorGeneral(collectorName, &MetricsCollectorOncall{})
		collectorGeneralList[collectorName].Run(opts.ScrapeTime.Live)
	} else {
		log.WithField("collector", collectorName).Infof("collector disabled")
	}

	collectorName = "Incident"
	if opts.ScrapeTime.Live.Seconds() > 0 {
		collectorGeneralList[collectorName] = NewCollectorGeneral(collectorName, &MetricsCollectorIncident{teamListOpt: opts.PagerDuty.Teams.Filter})
		collectorGeneralList[collectorName].Run(opts.ScrapeTime.Live)
	} else {
		log.WithField("collector", collectorName).Infof("collector disabled")
	}

	collectorName = "Summary"
	if opts.ScrapeTime.Summary.Seconds() > 0 {
		collectorGeneralList[collectorName] = NewCollectorGeneral(collectorName, &MetricsCollectorSummary{teamListOpt: opts.PagerDuty.Teams.Filter})
		collectorGeneralList[collectorName].Run(opts.ScrapeTime.Summary)
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

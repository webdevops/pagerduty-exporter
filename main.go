package main

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/jessevdk/go-flags"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"github.com/webdevops/go-common/prometheus/collector"

	"github.com/webdevops/pagerduty-exporter/config"
)

const (
	author = "webdevops.io"

	// PagerdutyListLimit limits the amount of items returned from an API query
	PagerdutyListLimit = 100
)

var (
	argparser *flags.Parser
	opts      config.Opts

	PagerDutyClient               *pagerduty.Client
	PrometheusPagerDutyApiCounter *prometheus.CounterVec

	// Git version information
	gitCommit = "<unknown>"
	gitTag    = "<unknown>"
)

func main() {
	initArgparser()
	initLogger()

	log.Infof("starting pagerduty-exporter v%s (%s; %s; by %v)", gitTag, gitCommit, runtime.Version(), author)
	log.Info(string(opts.GetJson()))

	log.Infof("init PagerDuty client")
	initPagerDuty()

	log.Infof("starting metrics collection")
	initMetricCollector()

	log.Infof("starting http server on %s", opts.ServerBind)
	startHTTPServer()
}

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

func initLogger() {
	// verbose level
	if opts.Logger.Debug {
		log.SetLevel(log.DebugLevel)
	}

	// trace level
	if opts.Logger.Trace {
		log.SetReportCaller(true)
		log.SetLevel(log.TraceLevel)
		log.SetFormatter(&log.TextFormatter{
			CallerPrettyfier: func(f *runtime.Frame) (string, string) {
				s := strings.Split(f.Function, "/")
				funcName := s[len(s)-1]
				return funcName, fmt.Sprintf("%s:%d", f.File, f.Line)
			},
		})
	}

	// json log format
	if opts.Logger.Json {
		log.SetReportCaller(true)
		log.SetFormatter(&log.JSONFormatter{
			DisableTimestamp: true,
			CallerPrettyfier: func(f *runtime.Frame) (string, string) {
				s := strings.Split(f.Function, "/")
				funcName := s[len(s)-1]
				return funcName, fmt.Sprintf("%s:%d", f.File, f.Line)
			},
		})
	}
}

// Init and build PagerDuty client
func initPagerDuty() {
	PagerDutyClient = pagerduty.NewClient(opts.PagerDuty.AuthToken)

	httpClientTransportProxy := http.ProxyFromEnvironment
	if opts.Logger.Debug {
		httpClientTransportProxy = pagerdutyRequestLogger
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

	PrometheusPagerDutyApiCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pagerduty_api_counter",
			Help: "Pagerduty api counter",
		},
		[]string{
			"name",
		},
	)
	prometheus.MustRegister(PrometheusPagerDutyApiCounter)
}

func initMetricCollector() {
	var collectorName string

	if !opts.PagerDuty.Teams.Disable {
		collectorName = "Team"
		if opts.ScrapeTime.General.Seconds() > 0 {
			c := collector.New(collectorName, &MetricsCollectorTeam{}, log.StandardLogger())
			c.SetScapeTime(opts.ScrapeTime.General)
			if err := c.Start(); err != nil {
				log.Panic(err.Error())
			}
		} else {
			log.WithField("collector", collectorName).Infof("collector disabled")
		}
	}

	collectorName = "User"
	if opts.ScrapeTime.General.Seconds() > 0 {
		c := collector.New(collectorName, &MetricsCollectorUser{teamListOpt: opts.PagerDuty.Teams.Filter}, log.StandardLogger())
		c.SetScapeTime(opts.ScrapeTime.General)
		if err := c.Start(); err != nil {
			log.Panic(err.Error())
		}
	} else {
		log.WithField("collector", collectorName).Infof("collector disabled")
	}

	collectorName = "Service"
	if opts.ScrapeTime.General.Seconds() > 0 {
		c := collector.New(collectorName, &MetricsCollectorService{teamListOpt: opts.PagerDuty.Teams.Filter}, log.StandardLogger())
		c.SetScapeTime(opts.ScrapeTime.General)
		if err := c.Start(); err != nil {
			log.Panic(err.Error())
		}
	} else {
		log.WithField("collector", collectorName).Infof("collector disabled")

	}

	collectorName = "Schedule"
	if opts.ScrapeTime.General.Seconds() > 0 {
		c := collector.New(collectorName, &MetricsCollectorSchedule{}, log.StandardLogger())
		c.SetScapeTime(opts.ScrapeTime.General)
		if err := c.Start(); err != nil {
			log.Panic(err.Error())
		}
	} else {
		log.WithField("collector", collectorName).Infof("collector disabled")
	}

	collectorName = "MaintenanceWindow"
	if opts.ScrapeTime.General.Seconds() > 0 {
		c := collector.New(collectorName, &MetricsCollectorMaintenanceWindow{teamListOpt: opts.PagerDuty.Teams.Filter}, log.StandardLogger())
		c.SetScapeTime(opts.ScrapeTime.General)
		if err := c.Start(); err != nil {
			log.Panic(err.Error())
		}
	} else {
		log.WithField("collector", collectorName).Infof("collector disabled")
	}

	collectorName = "OnCall"
	if opts.ScrapeTime.Live.Seconds() > 0 {
		c := collector.New(collectorName, &MetricsCollectorOncall{}, log.StandardLogger())
		c.SetScapeTime(opts.ScrapeTime.Live)
		if err := c.Start(); err != nil {
			log.Panic(err.Error())
		}
	} else {
		log.WithField("collector", collectorName).Infof("collector disabled")
	}

	collectorName = "Incident"
	if opts.ScrapeTime.Live.Seconds() > 0 {
		c := collector.New(collectorName, &MetricsCollectorIncident{teamListOpt: opts.PagerDuty.Teams.Filter}, log.StandardLogger())
		c.SetScapeTime(opts.ScrapeTime.Live)
		if err := c.Start(); err != nil {
			log.Panic(err.Error())
		}
	} else {
		log.WithField("collector", collectorName).Infof("collector disabled")
	}

	collectorName = "Summary"
	if opts.ScrapeTime.Summary.Seconds() > 0 {
		c := collector.New(collectorName, &MetricsCollectorSummary{teamListOpt: opts.PagerDuty.Teams.Filter}, log.StandardLogger())
		c.SetScapeTime(opts.ScrapeTime.Summary)
		if err := c.Start(); err != nil {
			log.Panic(err.Error())
		}
	} else {
		log.WithField("collector", collectorName).Infof("collector disabled")
	}
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

func pagerdutyRequestLogger(req *http.Request) (*url.URL, error) {
	log.Debugf("send request to %v", req.URL.String())
	return http.ProxyFromEnvironment(req)
}

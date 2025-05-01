package main

import (
	"errors"
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
	"github.com/webdevops/go-common/prometheus/collector"
	"go.uber.org/zap"

	"github.com/webdevops/pagerduty-exporter/config"
)

const (
	author = "webdevops.io"

	// PagerdutyListLimit limits the amount of items returned from an API query
	PagerdutyListLimit = 100
)

var (
	argparser *flags.Parser
	Opts      config.Opts

	PagerDutyClient               *pagerduty.Client
	PrometheusPagerDutyApiCounter *prometheus.CounterVec

	// Git version information
	gitCommit = "<unknown>"
	gitTag    = "<unknown>"
)

func main() {
	initArgparser()
	initLogger()

	logger.Infof("starting pagerduty-exporter v%s (%s; %s; by %v)", gitTag, gitCommit, runtime.Version(), author)
	logger.Info(string(Opts.GetJson()))
	initSystem()

	logger.Infof("init PagerDuty client")
	initPagerDuty()

	logger.Infof("starting metrics collection")
	initMetricCollector()

	logger.Infof("starting http server on %s", Opts.Server.Bind)
	startHTTPServer()
}

func initArgparser() {
	argparser = flags.NewParser(&Opts, flags.Default)
	_, err := argparser.Parse()

	// check if there is an parse error
	if err != nil {
		var flagsErr *flags.Error
		if ok := errors.As(err, &flagsErr); ok && flagsErr.Type == flags.ErrHelp {
			os.Exit(0)
		} else {
			fmt.Println()
			argparser.WriteHelp(os.Stdout)
			os.Exit(1)
		}
	}

	// Load the AuthTokenFile into the AuthToken with some validation
	if Opts.PagerDuty.AuthTokenFile != "" {
		data, err := os.ReadFile(Opts.PagerDuty.AuthTokenFile)
		if err != nil {
			logger.Fatalf("failed to read token from file: %v", err.Error())
		}
		Opts.PagerDuty.AuthToken = strings.TrimSpace(string(data))
	}

	if Opts.PagerDuty.AuthToken == "" {
		fmt.Println("ERROR: An authtoken or an authtokenfile must be specified")
		argparser.WriteHelp(os.Stdout)
		os.Exit(1)
	}

	if len(Opts.PagerDuty.Incident.Statuses) == 1 {
		if strings.ToLower(Opts.PagerDuty.Incident.Statuses[0]) == "all" {
			Opts.PagerDuty.Incident.Statuses = []string{
				"triggered",
				"acknowledged",
				"resolved",
			}
		}
	}

	if Opts.ScrapeTime.MaintenanceWindow == nil {
		Opts.ScrapeTime.MaintenanceWindow = &Opts.ScrapeTime.General
	}

	if Opts.ScrapeTime.Schedule == nil {
		Opts.ScrapeTime.Schedule = &Opts.ScrapeTime.General
	}

	if Opts.ScrapeTime.Service == nil {
		Opts.ScrapeTime.Service = &Opts.ScrapeTime.General
	}
	if Opts.ScrapeTime.Team == nil {
		Opts.ScrapeTime.Team = &Opts.ScrapeTime.General
	}

	if Opts.ScrapeTime.User == nil {
		Opts.ScrapeTime.User = &Opts.ScrapeTime.General
	}
}

// Init and build PagerDuty client
func initPagerDuty() {
	PagerDutyClient = pagerduty.NewClient(Opts.PagerDuty.AuthToken)

	httpClientTransportProxy := http.ProxyFromEnvironment
	if Opts.Logger.Debug {
		httpClientTransportProxy = pagerdutyRequestLogger
	}

	PagerDutyClient.HTTPClient = &http.Client{
		Transport: &http.Transport{
			Proxy: httpClientTransportProxy,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxConnsPerHost:       Opts.PagerDuty.MaxConnections,
			MaxIdleConns:          Opts.PagerDuty.MaxConnections,
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

	cacheTag := collector.BuildCacheTag(gitTag, Opts.PagerDuty)

	if !Opts.PagerDuty.Teams.Disable {
		collectorName = "Team"
		if Opts.ScrapeTime.Team.Seconds() > 0 {
			c := collector.New(collectorName, &MetricsCollectorTeam{}, logger)
			c.SetScapeTime(*Opts.ScrapeTime.Team)
			c.SetCache(Opts.GetCachePath("team.json"), cacheTag)
			if err := c.Start(); err != nil {
				logger.Panic(err.Error())
			}
		} else {
			logger.With(zap.String("collector", collectorName)).Infof("collector disabled")
		}
	}

	collectorName = "User"
	if Opts.ScrapeTime.User.Seconds() > 0 {
		c := collector.New(collectorName, &MetricsCollectorUser{teamListOpt: Opts.PagerDuty.Teams.Filter}, logger)
		c.SetScapeTime(*Opts.ScrapeTime.User)
		c.SetCache(Opts.GetCachePath("user.json"), cacheTag)
		if err := c.Start(); err != nil {
			logger.Panic(err.Error())
		}
	} else {
		logger.With(zap.String("collector", collectorName)).Infof("collector disabled")
	}

	collectorName = "Service"
	if Opts.ScrapeTime.Service.Seconds() > 0 {
		c := collector.New(collectorName, &MetricsCollectorService{teamListOpt: Opts.PagerDuty.Teams.Filter}, logger)
		c.SetScapeTime(*Opts.ScrapeTime.Service)
		c.SetCache(Opts.GetCachePath("service.json"), cacheTag)
		if err := c.Start(); err != nil {
			logger.Panic(err.Error())
		}
	} else {
		logger.With(zap.String("collector", collectorName)).Infof("collector disabled")

	}

	collectorName = "Schedule"
	if Opts.ScrapeTime.Schedule.Seconds() > 0 {
		c := collector.New(collectorName, &MetricsCollectorSchedule{}, logger)
		c.SetScapeTime(*Opts.ScrapeTime.Schedule)
		c.SetCache(Opts.GetCachePath("schedule.json"), cacheTag)
		if err := c.Start(); err != nil {
			logger.Panic(err.Error())
		}
	} else {
		logger.With(zap.String("collector", collectorName)).Infof("collector disabled")
	}

	collectorName = "MaintenanceWindow"
	if Opts.ScrapeTime.MaintenanceWindow.Seconds() > 0 {
		c := collector.New(collectorName, &MetricsCollectorMaintenanceWindow{teamListOpt: Opts.PagerDuty.Teams.Filter}, logger)
		c.SetScapeTime(*Opts.ScrapeTime.MaintenanceWindow)
		c.SetCache(Opts.GetCachePath("maintenancewindow.json"), cacheTag)
		if err := c.Start(); err != nil {
			logger.Panic(err.Error())
		}
	} else {
		logger.With(zap.String("collector", collectorName)).Infof("collector disabled")
	}

	collectorName = "OnCall"
	if Opts.ScrapeTime.Live.Seconds() > 0 {
		c := collector.New(collectorName, &MetricsCollectorOncall{}, logger)
		c.SetScapeTime(Opts.ScrapeTime.Live)
		c.SetCache(Opts.GetCachePath("oncall.json"), cacheTag)
		if err := c.Start(); err != nil {
			logger.Panic(err.Error())
		}
	} else {
		logger.With(zap.String("collector", collectorName)).Infof("collector disabled")
	}

	collectorName = "Incident"
	if Opts.ScrapeTime.Live.Seconds() > 0 {
		c := collector.New(collectorName, &MetricsCollectorIncident{teamListOpt: Opts.PagerDuty.Teams.Filter}, logger)
		c.SetScapeTime(Opts.ScrapeTime.Live)
		c.SetCache(Opts.GetCachePath("incident.json"), cacheTag)
		if err := c.Start(); err != nil {
			logger.Panic(err.Error())
		}
	} else {
		logger.With(zap.String("collector", collectorName)).Infof("collector disabled")
	}

	collectorName = "Summary"
	if Opts.ScrapeTime.Summary.Seconds() > 0 {
		c := collector.New(collectorName, &MetricsCollectorSummary{teamListOpt: Opts.PagerDuty.Teams.Filter}, logger)
		c.SetScapeTime(Opts.ScrapeTime.Summary)
		c.SetCache(Opts.GetCachePath("summary.json"), cacheTag)
		if err := c.Start(); err != nil {
			logger.Panic(err.Error())
		}
	} else {
		logger.With(zap.String("collector", collectorName)).Infof("collector disabled")
	}

	collectorName = "System"
	if Opts.ScrapeTime.System.Seconds() > 0 {
		c := collector.New(collectorName, &MetricsCollectorSystem{}, logger)
		c.SetScapeTime(Opts.ScrapeTime.Summary)
		c.SetCache(Opts.GetCachePath("system.json"), cacheTag)
		if err := c.Start(); err != nil {
			logger.Panic(err.Error())
		}
	} else {
		logger.With(zap.String("collector", collectorName)).Infof("collector disabled")
	}
}

// start and handle prometheus handler
func startHTTPServer() {
	mux := http.NewServeMux()

	// healthz
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if _, err := fmt.Fprint(w, "Ok"); err != nil {
			logger.Error(err)
		}
	})

	// readyz
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if _, err := fmt.Fprint(w, "Ok"); err != nil {
			logger.Error(err)
		}
	})

	mux.Handle("/metrics", promhttp.Handler())

	srv := &http.Server{
		Addr:         Opts.Server.Bind,
		Handler:      mux,
		ReadTimeout:  Opts.Server.ReadTimeout,
		WriteTimeout: Opts.Server.WriteTimeout,
	}
	logger.Fatal(srv.ListenAndServe())
}

func pagerdutyRequestLogger(req *http.Request) (*url.URL, error) {
	logger.Debugf("send request to %v", req.URL.String())
	return http.ProxyFromEnvironment(req)
}

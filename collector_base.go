package main

import (
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"sync"
	"time"
)

var collectorGlobal CollectorGlobal

type CollectorBase struct {
	Name       string
	scrapeTime *time.Duration

	LastScrapeDuration  *time.Duration
	collectionStartTime time.Time

	logger *log.Entry

	isHidden bool
}

type CollectorGlobal struct {
	prometheus struct {
		stats      *prometheus.GaugeVec
		statsMutex sync.Mutex

		api      *prometheus.CounterVec
		apiMutex sync.Mutex
	}
}

func (c *CollectorBase) Init() {
	c.isHidden = false
	c.logger = log.WithField("collector", c.Name)
}

func (c *CollectorBase) SetScrapeTime(scrapeTime time.Duration) {
	c.scrapeTime = &scrapeTime
}

func (c *CollectorBase) GetScrapeTime() *time.Duration {
	return c.scrapeTime
}

func (c *CollectorBase) SetIsHidden(v bool) {
	c.isHidden = v
}

func (c *CollectorBase) PrometheusStatsGauge() *prometheus.GaugeVec {
	if collectorGlobal.prometheus.stats == nil {
		collectorGlobal.prometheus.statsMutex.Lock()

		collectorGlobal.prometheus.stats = prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "pagerduty_stats",
				Help: "Pagerduty statistics",
			},
			[]string{
				"name",
				"type",
			},
		)

		prometheus.MustRegister(collectorGlobal.prometheus.stats)
		collectorGlobal.prometheus.statsMutex.Unlock()
	}

	return collectorGlobal.prometheus.stats
}

func (c *CollectorBase) PrometheusAPICounter() *prometheus.CounterVec {
	if collectorGlobal.prometheus.api == nil {
		collectorGlobal.prometheus.apiMutex.Lock()

		collectorGlobal.prometheus.api = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "pagerduty_api_counter",
				Help: "Pagerduty api counter",
			},
			[]string{
				"name",
			},
		)

		prometheus.MustRegister(collectorGlobal.prometheus.api)
		collectorGlobal.prometheus.apiMutex.Unlock()
	}

	return collectorGlobal.prometheus.api
}

func (c *CollectorBase) collectionStart() {
	c.collectionStartTime = time.Now()

	if !c.isHidden {
		log.Infof("collector[%s]: starting metrics collection", c.Name)
	}
}

func (c *CollectorBase) collectionFinish() {
	duration := time.Now().Sub(c.collectionStartTime)
	c.LastScrapeDuration = &duration

	if !c.isHidden {
		c.logger.WithField("duration", c.LastScrapeDuration).Infof("finished metrics collection (duration: %v)", c.LastScrapeDuration)
	}
}

func (c *CollectorBase) sleepUntilNextCollection() {
	if !c.isHidden {
		c.logger.Debugf("sleeping %v", c.GetScrapeTime().String())
	}
	time.Sleep(*c.GetScrapeTime())
}

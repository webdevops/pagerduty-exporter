package main

import (
	"context"
	"github.com/prometheus/client_golang/prometheus"
)

type MetricsCollectorCollector struct {
	CollectorProcessorGeneral
}

func (m *MetricsCollectorCollector) Setup(collector *CollectorGeneral) {
	m.CollectorReference = collector
}

func (m *MetricsCollectorCollector) Reset() {
}

func (m *MetricsCollectorCollector) Collect(ctx context.Context, callback chan<- func()) {
	m.collectCollectorStats(ctx, callback)
}

func (m *MetricsCollectorCollector) collectCollectorStats(ctx context.Context, callback chan<- func()) {
	statsMetrics := MetricCollectorList{}

	for _, collector := range collectorGeneralList {
		if collector.LastScrapeDuration != nil {
			statsMetrics.AddDuration(prometheus.Labels{
				"name": collector.Name,
				"type": "collectorDuration",
			}, *collector.LastScrapeDuration)
		}
	}

	callback <- func() {
		statsMetrics.GaugeSet(m.CollectorReference.PrometheusStatsGauge())
	}
}

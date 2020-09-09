package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

func createIpedMetrics() ipedMetrics {
	return ipedMetrics{
		calls: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "ipedworker_runIped_calls",
			Help: "Number of calls to runIped",
		}, []string{"hostname", "evidence"}),
		finish: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "ipedworker_runIped_finish",
			Help: "Number of finished runs",
		}, []string{"hostname", "evidence", "result"}),
		running: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "ipedworker_runIped_running",
			Help: "Whether IPED is running or not",
		}, []string{"hostname", "evidence"}),
		found: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "ipedworker_runIped_found",
			Help: "Number of items found",
		}, []string{"hostname", "evidence"}),
		processed: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "ipedworker_runIped_processed",
			Help: "Number of items processed",
		}, []string{"hostname", "evidence"}),
	}
}

type ipedMetrics struct {
	calls     *prometheus.CounterVec
	finish    *prometheus.CounterVec
	running   *prometheus.GaugeVec
	found     *prometheus.GaugeVec
	processed *prometheus.GaugeVec
}

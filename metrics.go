package main

import "github.com/prometheus/client_golang/prometheus"

type MetricsEngine struct {
	events         prometheus.Counter
	processingTime prometheus.Histogram
	reconnects     prometheus.Counter
}

func (m *MetricsEngine) Init() {
	m.events = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "kubecord_ws_events",
			Help: "Total number of events received",
		})

	m.processingTime = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name: "kubecord_ws_processing_time",
			Help: "Processing time for events",
		})
	m.reconnects = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "kubecord_ws_reconnects",
			Help: "Total number of reconnects",
		})

	prometheus.MustRegister(m.events)
	prometheus.MustRegister(m.processingTime)
	prometheus.MustRegister(m.reconnects)
}

package msgpipeline

import "github.com/prometheus/client_golang/prometheus"

var (
	checkReject = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "maddy",
			Subsystem: "check",
			Name:      "reject",
			Help:      "Number of times a check returned 'reject' result (may be more than processed messages if check does so on per-recipient basis)",
		},
		[]string{"check"},
	)
	checkQuarantined = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "maddy",
			Subsystem: "check",
			Name:      "quarantined",
			Help:      "Number of times a check returned 'quarantine' result (may be more than processed messages if check does so on per-recipient basis)",
		},
		[]string{"check"},
	)
)

func init() {
	prometheus.MustRegister(checkReject)
	prometheus.MustRegister(checkQuarantined)
}

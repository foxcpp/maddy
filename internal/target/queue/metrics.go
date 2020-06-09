package queue

import "github.com/prometheus/client_golang/prometheus"

var queuedMsgs = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Namespace: "maddy",
		Subsystem: "queue",
		Name:      "length",
		Help:      "Amount of queued messages",
	},
	[]string{"module", "location"},
)

func init() {
	prometheus.MustRegister(queuedMsgs)
}

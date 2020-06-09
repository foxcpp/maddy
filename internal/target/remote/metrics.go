package remote

import "github.com/prometheus/client_golang/prometheus"

var mxLevelCnt = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Namespace: "maddy",
		Subsystem: "remote",
		Name:      "conns_mx_level",
		Help:      "Outbound connections established with specific MX security level",
	},
	[]string{"module", "level"},
)

var tlsLevelCnt = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Namespace: "maddy",
		Subsystem: "remote",
		Name:      "conns_tls_level",
		Help:      "Outbound connections established with specific TLS security level",
	},
	[]string{"module", "level"},
)

func init() {
	prometheus.MustRegister(mxLevelCnt)
	prometheus.MustRegister(tlsLevelCnt)
}

package smtp

import "github.com/prometheus/client_golang/prometheus"

var (
	startedSMTPTransactions = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "maddy",
			Subsystem: "smtp",
			Name:      "started_transactions",
			Help:      "Amount of SMTP trasanactions started",
		},
		[]string{"module"},
	)
	completedSMTPTransactions = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "maddy",
			Subsystem: "smtp",
			Name:      "smtp_completed_transactions",
			Help:      "Amount of SMTP trasanactions succesfully completed",
		},
		[]string{"module"},
	)
	abortedSMTPTransactions = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "maddy",
			Subsystem: "smtp",
			Name:      "aborted_transactions",
			Help:      "Amount of SMTP trasanactions aborted",
		},
		[]string{"module"},
	)

	ratelimitDefers = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "maddy",
			Subsystem: "smtp",
			Name:      "ratelimit_deferred",
			Help:      "Messages rejected with 4xx code due to ratelimiting",
		},
		[]string{"module"},
	)
	failedLogins = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "maddy",
			Subsystem: "smtp",
			Name:      "failed_logins",
			Help:      "AUTH command failures",
		},
		[]string{"module"},
	)
	failedCmds = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "maddy",
			Subsystem: "smtp",
			Name:      "failed_commands",
			Help:      "Messages rejected with 4xx code due to ratelimiting",
		},
		[]string{"module", "command", "smtp_code", "smtp_enchcode"},
	)
)

func init() {
	prometheus.MustRegister(startedSMTPTransactions)
	prometheus.MustRegister(completedSMTPTransactions)
	prometheus.MustRegister(abortedSMTPTransactions)
	prometheus.MustRegister(ratelimitDefers)
	prometheus.MustRegister(failedCmds)
}

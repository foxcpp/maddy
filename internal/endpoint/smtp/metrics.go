/*
Maddy Mail Server - Composable all-in-one email server.
Copyright © 2019-2020 Max Mazurov <fox.cpp@disroot.org>, Maddy Mail Server contributors

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

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
			Help:      "Amount of SMTP trasanactions successfully completed",
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
			Help:      "Failed transaction commands (MAIL, RCPT, DATA)",
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

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

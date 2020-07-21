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

package auth

import "strings"

func CheckDomainAuth(username string, perDomain bool, allowedDomains []string) (loginName string, allowed bool) {
	var accountName, domain string
	if perDomain {
		parts := strings.Split(username, "@")
		if len(parts) != 2 {
			return "", false
		}
		domain = parts[1]
		accountName = username
	} else {
		parts := strings.Split(username, "@")
		accountName = parts[0]
		if len(parts) == 2 {
			domain = parts[1]
		}
	}

	allowed = domain == ""
	if allowedDomains != nil && domain != "" {
		for _, allowedDomain := range allowedDomains {
			if strings.EqualFold(domain, allowedDomain) {
				allowed = true
			}
		}
		if !allowed {
			return "", false
		}
	}

	return accountName, allowed
}

package maddy

import "strings"

func checkDomainAuth(username string, perDomain bool, allowedDomains []string) (loginName string, allowed bool) {
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

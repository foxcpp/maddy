package mtasts

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func downloadPolicy(domain string) (*Policy, error) {
	// TODO: Consult OCSP/CRL to detect revoked certificates?

	resp, err := http.Get("https://mta-sts." + domain + "/.well-known/mta-sts.txt")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Policies fetched via HTTPS are only valid if the HTTP response code is
	// 200 (OK).  HTTP 3xx redirects MUST NOT be followed.
	if resp.StatusCode != 200 {
		return nil, errors.New("mtasts: HTTP " + resp.Status)
	}

	if !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/plain") {
		return nil, errors.New("mtasts: unexpected content type")
	}

	return readPolicy(resp.Body)
}

// Cache structure implements transparent MTA-STS policy caching using FS
// directory.
type Cache struct {
	Location string
}

var ErrNoPolicy = errors.New("mtasts: no MTA-STS policy found")

// Get reads policy from cache or tries to fetch it from Policy Host.
func (c *Cache) Get(domain string) (*Policy, error) {
	validCache := true
	cachedId, fetchTime, cachedPolicy, err := c.load(domain)
	if err != nil {
		if !os.IsNotExist(err) {
			// Something wrong with FS directory used for caching, this is bad.
			return nil, err
		}

		validCache = false
	} else if fetchTime.Add(time.Duration(cachedPolicy.MaxAge) * time.Second).Before(time.Now()) {
		validCache = false
	}

	records, err := net.LookupTXT("_mta-sts." + domain)
	if err != nil {
		if derr, ok := err.(*net.DNSError); ok && !derr.IsTemporary {
			return nil, ErrNoPolicy
		}
		return nil, err
	}

	// RFC says:
	//   If the number of resulting records is not one, or if the resulting
	//   record is syntactically invalid, senders MUST assume the recipient
	//   domain does not have an available MTA-STS Policy. ...
	//   (Note that the absence of a usable TXT record is not by itself
	//   sufficient to remove a sender's previously cached policy for the Policy
	//   Domain, as discussed in Section 5.1, "Policy Application Control Flow".)
	if len(records) != 1 {
		if validCache {
			return cachedPolicy, nil
		}
		return nil, ErrNoPolicy
	}
	dnsId, err := readDNSRecord(records[0])
	if err != nil {
		if validCache {
			return cachedPolicy, nil
		}
		return nil, ErrNoPolicy
	}

	if !validCache || dnsId != cachedId {
		policy, err := downloadPolicy(domain)
		if err != nil {
			return nil, err
		}

		if err := c.store(domain, dnsId, time.Now(), policy); err != nil {
			// We still got up-to-date policy, cache is not critcial.
			return policy, nil
		}
		return policy, nil
	}

	return cachedPolicy, nil
}

func (c *Cache) store(domain, id string, fetchTime time.Time, p *Policy) error {
	f, err := os.Create(filepath.Join(c.Location, domain))
	if err != nil {
		return err
	}
	defer f.Close()

	return json.NewEncoder(f).Encode(map[string]interface{}{
		"ID":        id,
		"FetchTime": fetchTime,
		"Policy":    p,
	})
}

func (c *Cache) load(domain string) (id string, fetchTime time.Time, p *Policy, err error) {
	f, err := os.Open(filepath.Join(c.Location, domain))
	if err != nil {
		return "", time.Time{}, nil, err
	}
	defer f.Close()

	data := struct {
		ID        string
		FetchTime time.Time
		Policy    *Policy
	}{}
	if err := json.NewDecoder(f).Decode(&data); err != nil {
		return "", time.Time{}, nil, err
	}
	return data.ID, data.FetchTime, data.Policy, nil
}

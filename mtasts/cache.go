package mtasts

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"mime"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/foxcpp/maddy/exterrors"
	"github.com/foxcpp/maddy/log"
)

var httpClient = &http.Client{
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return errors.New("mtasts: HTTP redirects are forbidden")
	},
	Timeout: time.Minute,
}

func downloadPolicy(domain string) (*Policy, error) {
	// TODO: Consult OCSP/CRL to detect revoked certificates?

	resp, err := httpClient.Get("https://mta-sts." + domain + "/.well-known/mta-sts.txt")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Policies fetched via HTTPS are only valid if the HTTP response code is
	// 200 (OK).  HTTP 3xx redirects MUST NOT be followed.
	if resp.StatusCode != 200 {
		return nil, errors.New("mtasts: HTTP " + resp.Status)
	}

	contentType, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil {
		return nil, err
	}

	if contentType != "text/plain" {
		return nil, errors.New("mtasts: unexpected content type")
	}

	return readPolicy(resp.Body)
}

type Resolver interface {
	LookupTXT(ctx context.Context, domain string) ([]string, error)
}

// Cache structure implements transparent MTA-STS policy caching using FS
// directory.
type Cache struct {
	Location string
	Resolver Resolver
	Logger   log.Logger

	// Hook for testing, if non-nil replaces the downloadPolicy implementation
	// above.
	downloadPolicy func(string) (*Policy, error)
}

func IsNoPolicy(err error) bool {
	return err == ErrIgnorePolicy
}

// ErrIgnorePolicy indicates that remote domain offers an MTA-STS policy,
// but it is ignored due to some error.
//
// Callers should not check for this directly and use IsNoPolicy
// function to decide actual handling strategy.
var ErrIgnorePolicy = errors.New("mtasts: policy ignored due to errors")

// Get reads policy from cache or tries to fetch it from Policy Host.
func (c *Cache) Get(domain string) (*Policy, error) {
	_, p, err := c.fetch(false, time.Now(), domain)
	return p, err
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

func (c *Cache) Refresh() error {
	dir, err := ioutil.ReadDir(c.Location)
	if err != nil {
		return err
	}

	for _, ent := range dir {
		if ent.IsDir() {
			continue
		}
		// If policy is going to expire in next 6 hours (half of our refresh
		// period) - we still want to refresh it.
		// Since otherwise we are going to have expired policy for another 6 hours,
		// which makes it useless.
		// See https://tools.ietf.org/html/rfc8461#section-10.2.
		cacheHit, _, err := c.fetch(true, time.Now().Add(6*time.Hour), ent.Name())
		if err != nil && err != ErrIgnorePolicy {
			c.Logger.Error("policy update error", err, "domain", ent.Name())
		}
		if !cacheHit && err == nil {
			c.Logger.Debugln("updated MTA-STS policy for", ent.Name())
		}

		// TODO: figure out how to clean stale entires from cache
		// and if this is really necessary.
	}

	return nil
}

func (c *Cache) fetch(ignoreDns bool, now time.Time, domain string) (cacheHit bool, p *Policy, err error) {
	validCache := true
	cachedId, fetchTime, cachedPolicy, err := c.load(domain)
	if err != nil {
		if !os.IsNotExist(err) {
			// Something wrong with the FS directory used for caching, this is bad.
			return false, nil, err
		}

		validCache = false
	} else if fetchTime.Add(time.Duration(cachedPolicy.MaxAge) * time.Second).Before(now) {
		validCache = false
	}

	var dnsId string
	if !ignoreDns {
		records, err := c.Resolver.LookupTXT(context.Background(), "_mta-sts."+domain)
		if err != nil {
			if validCache {
				if dnsErr, ok := err.(*net.DNSError); ok && !dnsErr.IsNotFound {
					reason, misc := exterrors.UnwrapDNSErr(err)
					if reason != "" {
						misc["reason"] = reason
					}
					err = exterrors.WithFields(err, misc)
					c.Logger.Error("failed lookup the DNS record, using cache", err, "domain", domain)
				}
				return true, cachedPolicy, nil
			}

			if derr, ok := err.(*net.DNSError); ok && !derr.IsTemporary {
				if dnsErr, ok := err.(*net.DNSError); ok && !dnsErr.IsNotFound {
					reason, misc := exterrors.UnwrapDNSErr(err)
					if reason != "" {
						misc["reason"] = reason
					}
					err = exterrors.WithFields(err, misc)
					c.Logger.Error("failed lookup the DNS record, ignoring", err, "domain", domain)
				}
				return false, nil, ErrIgnorePolicy
			}
			return false, nil, err
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
				c.Logger.Msg("multiple DNS records, using cache", "domain", domain)
				return true, cachedPolicy, nil
			}
			c.Logger.Msg("multiple DNS records, ignoring", "domain", domain)
			return false, nil, ErrIgnorePolicy
		}
		dnsId, err = readDNSRecord(records[0])
		if err != nil {
			if validCache {
				c.Logger.Error("failed read the DNS record, using cache", err, "domain", domain)
				return true, cachedPolicy, nil
			}
			c.Logger.Error("failed read the DNS record, ignoring", err, "domain", domain)
			return false, nil, ErrIgnorePolicy
		}
	}

	if !validCache || dnsId != cachedId {
		download := downloadPolicy
		if c.downloadPolicy != nil {
			download = c.downloadPolicy
		}
		policy, err := download(domain)
		if err != nil {
			if validCache {
				c.Logger.Error("failed to fetch new policy, using cache", err, "domain", domain)
				return true, cachedPolicy, nil
			}
			c.Logger.Error("failed to fetch new policy, ignoring", err, "domain", domain)
			return false, nil, ErrIgnorePolicy
		}

		if err := c.store(domain, dnsId, time.Now(), policy); err != nil {
			c.Logger.Error("failed to store the new policy", err, "domain", domain)
			// We still got up-to-date policy, cache is not critcial.
			return false, cachedPolicy, nil
		}
		return false, policy, nil
	}

	return true, cachedPolicy, nil
}

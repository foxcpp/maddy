package mtasts

import (
	"context"
	"errors"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/foxcpp/go-mockdns"
	"github.com/foxcpp/maddy/internal/testutils"
)

func mockDownloadPolicy(policy *Policy, err error) func(string) (*Policy, error) {
	return func(string) (*Policy, error) {
		return policy, err
	}
}

func TestCacheGet(t *testing.T) {
	expectedPolicy := &Policy{
		Mode:   ModeEnforce,
		MaxAge: 60,
		MX:     []string{"a"},
	}
	c := Cache{
		Location: testutils.Dir(t),
		Resolver: &mockdns.Resolver{
			Zones: map[string]mockdns.Zone{
				"_mta-sts.example.org.": {
					TXT: []string{"v=STSv1; id=1234"},
				},
			},
		},
		Logger:         testutils.Logger(t, "mtasts"),
		downloadPolicy: mockDownloadPolicy(expectedPolicy, nil),
	}
	defer os.RemoveAll(c.Location)

	policy, err := c.Get(context.Background(), "example.org")
	if err != nil {
		t.Fatalf("policy get: %v", err)
	}
	if !reflect.DeepEqual(policy, expectedPolicy) {
		t.Fatalf("wrong policy returned, want %+v, got %+v", expectedPolicy, policy)
	}
}

func TestCacheGet_Error_DNS(t *testing.T) {
	c := Cache{
		Location: testutils.Dir(t),
		Resolver: &mockdns.Resolver{
			Zones: nil,
		},
		Logger:         testutils.Logger(t, "mtasts"),
		downloadPolicy: mockDownloadPolicy(nil, errors.New("broken")),
	}
	defer os.RemoveAll(c.Location)

	_, err := c.Get(context.Background(), "example.org")
	if err != ErrIgnorePolicy {
		t.Fatalf("policy get: %v", err)
	}
}

func TestCacheGet_Error_HTTPS(t *testing.T) {
	c := Cache{
		Location: testutils.Dir(t),
		Resolver: &mockdns.Resolver{
			Zones: map[string]mockdns.Zone{
				"_mta-sts.example.org.": {
					TXT: []string{"v=STSv1; id=1234"},
				},
			},
		},
		Logger:         testutils.Logger(t, "mtasts"),
		downloadPolicy: mockDownloadPolicy(nil, errors.New("broken")),
	}
	defer os.RemoveAll(c.Location)

	_, err := c.Get(context.Background(), "example.org")
	if err != ErrIgnorePolicy {
		t.Fatalf("policy get: %v", err)
	}
}

func TestCacheGet_Cached(t *testing.T) {
	expectedPolicy := &Policy{
		Mode:   ModeEnforce,
		MaxAge: 60,
		MX:     []string{"a"},
	}
	c := Cache{
		Location: testutils.Dir(t),
		Resolver: &mockdns.Resolver{
			Zones: map[string]mockdns.Zone{
				"_mta-sts.example.org.": {
					TXT: []string{"v=STSv1; id=1234"},
				},
			},
		},
		Logger:         testutils.Logger(t, "mtasts"),
		downloadPolicy: mockDownloadPolicy(expectedPolicy, nil),
	}
	defer os.RemoveAll(c.Location)

	policy, err := c.Get(context.Background(), "example.org")
	if err != nil {
		t.Fatalf("policy get: %v", err)
	}
	if !reflect.DeepEqual(policy, expectedPolicy) {
		t.Fatalf("wrong policy returned, want %+v, got %+v", expectedPolicy, policy)
	}
	// It should be cached up to 60 seconds, so second Get should work without
	// calling downloadPolicy.
	c.downloadPolicy = mockDownloadPolicy(nil, errors.New("broken"))

	policy, err = c.Get(context.Background(), "example.org")
	if err != nil {
		t.Fatalf("policy get: %v", err)
	}
	if !reflect.DeepEqual(policy, expectedPolicy) {
		t.Fatalf("wrong policy returned, want %+v, got %+v", expectedPolicy, policy)
	}
}

func TestCacheGet_Expired(t *testing.T) {
	t.Parallel()

	expectedPolicy := &Policy{
		Mode:   ModeEnforce,
		MaxAge: 1,
		MX:     []string{"a"},
	}
	c := Cache{
		Location: testutils.Dir(t),
		Resolver: &mockdns.Resolver{
			Zones: map[string]mockdns.Zone{
				"_mta-sts.example.org.": {
					TXT: []string{"v=STSv1; id=1234"},
				},
			},
		},
		Logger:         testutils.Logger(t, "mtasts"),
		downloadPolicy: mockDownloadPolicy(expectedPolicy, nil),
	}
	defer os.RemoveAll(c.Location)

	policy, err := c.Get(context.Background(), "example.org")
	if err != nil {
		t.Fatalf("policy get: %v", err)
	}
	if !reflect.DeepEqual(policy, expectedPolicy) {
		t.Fatalf("wrong policy returned, want %+v, got %+v", expectedPolicy, policy)
	}

	time.Sleep(2 * time.Second)

	// Policy should expire now. Next Get should refetch it.
	expectedPolicy.MX = []string{"b"}
	c.downloadPolicy = mockDownloadPolicy(expectedPolicy, nil)

	policy, err = c.Get(context.Background(), "example.org")
	if err != nil {
		t.Fatalf("policy get: %v", err)
	}
	if !reflect.DeepEqual(policy, expectedPolicy) {
		t.Fatalf("wrong policy returned, want %+v, got %+v", expectedPolicy, policy)
	}
}

func TestCacheGet_IDChange(t *testing.T) {
	expectedPolicy := &Policy{
		Mode:   ModeEnforce,
		MaxAge: 60,
		MX:     []string{"a"},
	}
	resolver := &mockdns.Resolver{
		Zones: map[string]mockdns.Zone{
			"_mta-sts.example.org.": {
				TXT: []string{"v=STSv1; id=1234"},
			},
		},
	}
	c := Cache{
		Location:       testutils.Dir(t),
		Resolver:       resolver,
		Logger:         testutils.Logger(t, "mtasts"),
		downloadPolicy: mockDownloadPolicy(expectedPolicy, nil),
	}
	defer os.RemoveAll(c.Location)

	policy, err := c.Get(context.Background(), "example.org")
	if err != nil {
		t.Fatalf("policy get: %v", err)
	}
	if !reflect.DeepEqual(policy, expectedPolicy) {
		t.Fatalf("wrong policy returned, want %+v, got %+v", expectedPolicy, policy)
	}

	// Policy ID change should cause policy refetch even if it is not expired
	// yet.
	resolver.Zones["_mta-sts.example.org."] = mockdns.Zone{
		TXT: []string{"v=STSv1; id=2345"},
	}
	expectedPolicy.MX = []string{"b"}
	c.downloadPolicy = mockDownloadPolicy(expectedPolicy, nil)

	policy, err = c.Get(context.Background(), "example.org")
	if err != nil {
		t.Fatalf("policy get: %v", err)
	}
	if !reflect.DeepEqual(policy, expectedPolicy) {
		t.Fatalf("wrong policy returned, want %+v, got %+v", expectedPolicy, policy)
	}
}

func TestCacheGet_DNSDisappear(t *testing.T) {
	expectedPolicy := &Policy{
		Mode:   ModeEnforce,
		MaxAge: 60,
		MX:     []string{"a"},
	}
	resolver := &mockdns.Resolver{
		Zones: map[string]mockdns.Zone{
			"_mta-sts.example.org.": {
				TXT: []string{"v=STSv1; id=1234"},
			},
		},
	}
	c := Cache{
		Location:       testutils.Dir(t),
		Resolver:       resolver,
		Logger:         testutils.Logger(t, "mtasts"),
		downloadPolicy: mockDownloadPolicy(expectedPolicy, nil),
	}
	defer os.RemoveAll(c.Location)

	policy, err := c.Get(context.Background(), "example.org")
	if err != nil {
		t.Fatalf("policy get: %v", err)
	}
	if !reflect.DeepEqual(policy, expectedPolicy) {
		t.Fatalf("wrong policy returned, want %+v, got %+v", expectedPolicy, policy)
	}

	// RFC 8461, Page 10:
	// >Conversely, if no "live" policy can be discovered via DNS or fetched
	// >via HTTPS, but a valid (non-expired) policy exists in the sender's
	// >cache, the sender MUST apply that cached policy.
	resolver.Zones = nil
	c.downloadPolicy = mockDownloadPolicy(nil, errors.New("broken"))

	policy, err = c.Get(context.Background(), "example.org")
	if err != nil {
		t.Fatalf("policy get: %v", err)
	}
	if !reflect.DeepEqual(policy, expectedPolicy) {
		t.Fatalf("wrong policy returned, want %+v, got %+v", expectedPolicy, policy)
	}
}

func TestCacheGet_HTTPGet_ErrNoPolicy(t *testing.T) {
	resolver := &mockdns.Resolver{
		Zones: map[string]mockdns.Zone{
			"_mta-sts.example.org.": {
				TXT: []string{"v=STSv1; id=1234"},
			},
		},
	}
	c := Cache{
		Location:       testutils.Dir(t),
		Resolver:       resolver,
		Logger:         testutils.Logger(t, "mtasts"),
		downloadPolicy: mockDownloadPolicy(nil, errors.New("broken")),
	}
	defer os.RemoveAll(c.Location)

	// RFC 8461, Page 10:
	// >If a valid TXT record is found but no policy can be fetched via HTTPS
	// >(for any reason), and there is no valid (non-expired) previously
	// >cached policy, senders MUST continue with delivery as though the
	// >domain has not implemented MTA-STS.
	_, err := c.Get(context.Background(), "example.org")
	if err != ErrIgnorePolicy {
		t.Fatalf("policy get: %v", err)
	}
}

func TestCacheGet_IDChange_Error(t *testing.T) {
	expectedPolicy := &Policy{
		Mode:   ModeEnforce,
		MaxAge: 60,
		MX:     []string{"a"},
	}
	resolver := &mockdns.Resolver{
		Zones: map[string]mockdns.Zone{
			"_mta-sts.example.org.": {
				TXT: []string{"v=STSv1; id=1234"},
			},
		},
	}
	c := Cache{
		Location:       testutils.Dir(t),
		Resolver:       resolver,
		Logger:         testutils.Logger(t, "mtasts"),
		downloadPolicy: mockDownloadPolicy(expectedPolicy, nil),
	}
	defer os.RemoveAll(c.Location)

	policy, err := c.Get(context.Background(), "example.org")
	if err != nil {
		t.Fatalf("policy get: %v", err)
	}
	if !reflect.DeepEqual(policy, expectedPolicy) {
		t.Fatalf("wrong policy returned, want %+v, got %+v", expectedPolicy, policy)
	}

	// Policy ID change should cause policy refetch even if it is not expired
	// yet.
	// ... however, if download of updated policy fails and there is cached one
	// - it should be used.
	resolver.Zones["_mta-sts.example.org."] = mockdns.Zone{
		TXT: []string{"v=STSv1; id=2345"},
	}
	c.downloadPolicy = mockDownloadPolicy(nil, errors.New("broken"))

	policy, err = c.Get(context.Background(), "example.org")
	if err != nil {
		t.Fatalf("policy get: %v", err)
	}
	if !reflect.DeepEqual(policy, expectedPolicy) {
		t.Fatalf("wrong policy returned, want %+v, got %+v", expectedPolicy, policy)
	}
}

func TestCacheGet_IDChange_Expired_Error(t *testing.T) {
	t.Parallel()

	expectedPolicy := &Policy{
		Mode:   ModeEnforce,
		MaxAge: 1,
		MX:     []string{"a"},
	}
	resolver := &mockdns.Resolver{
		Zones: map[string]mockdns.Zone{
			"_mta-sts.example.org.": {
				TXT: []string{"v=STSv1; id=1234"},
			},
		},
	}
	c := Cache{
		Location:       testutils.Dir(t),
		Resolver:       resolver,
		Logger:         testutils.Logger(t, "mtasts"),
		downloadPolicy: mockDownloadPolicy(expectedPolicy, nil),
	}
	defer os.RemoveAll(c.Location)

	policy, err := c.Get(context.Background(), "example.org")
	if err != nil {
		t.Fatalf("policy get: %v", err)
	}
	if !reflect.DeepEqual(policy, expectedPolicy) {
		t.Fatalf("wrong policy returned, want %+v, got %+v", expectedPolicy, policy)
	}

	time.Sleep(2 * time.Second)

	// Policy ID change should cause policy refetch even if it is not expired
	// yet.
	// ... however, if download of updated policy fails and there is cached one
	// - it should be used ... unless it is expired (it is, in this case).
	resolver.Zones["_mta-sts.example.org."] = mockdns.Zone{
		TXT: []string{"v=STSv1; id=2345"},
	}
	c.downloadPolicy = mockDownloadPolicy(nil, errors.New("broken"))

	policy, err = c.Get(context.Background(), "example.org")
	if err == nil {
		t.Fatalf("expected error, got policy %v", policy)
	}
}

func TestCacheRefresh(t *testing.T) {
	t.Parallel()

	expectedPolicy := &Policy{
		Mode:   ModeEnforce,
		MaxAge: 1,
		MX:     []string{"a"},
	}
	resolver := &mockdns.Resolver{
		Zones: map[string]mockdns.Zone{
			"_mta-sts.example.org.": {
				TXT: []string{"v=STSv1; id=1234"},
			},
		},
	}
	c := Cache{
		Location:       testutils.Dir(t),
		Resolver:       resolver,
		Logger:         testutils.Logger(t, "mtasts"),
		downloadPolicy: mockDownloadPolicy(expectedPolicy, nil),
	}
	defer os.RemoveAll(c.Location)

	policy, err := c.Get(context.Background(), "example.org")
	if err != nil {
		t.Fatalf("policy get: %v", err)
	}
	if !reflect.DeepEqual(policy, expectedPolicy) {
		t.Fatalf("wrong policy returned, want %+v, got %+v", expectedPolicy, policy)
	}

	time.Sleep(2 * time.Second)

	expectedPolicy.MX = []string{"b"}
	c.downloadPolicy = mockDownloadPolicy(expectedPolicy, nil)

	// It should fetch the new record.
	if err := c.Refresh(); err != nil {
		t.Fatalf("cache refresh: %v", err)
	}

	// Then don't allow Get to refetch the record.
	c.downloadPolicy = mockDownloadPolicy(nil, errors.New("broken"))

	// It should return the new record from cache.
	policy, err = c.Get(context.Background(), "example.org")
	if err != nil {
		t.Fatalf("policy get: %v", err)
	}
	if !reflect.DeepEqual(policy, expectedPolicy) {
		t.Fatalf("wrong policy returned, want %+v, got %+v", expectedPolicy, policy)
	}
}

func TestCacheRefresh_Error(t *testing.T) {
	t.Parallel()

	expectedPolicy := &Policy{
		Mode:   ModeEnforce,
		MaxAge: 60,
		MX:     []string{"a"},
	}
	resolver := &mockdns.Resolver{
		Zones: map[string]mockdns.Zone{
			"_mta-sts.example.org.": {
				TXT: []string{"v=STSv1; id=1234"},
			},
		},
	}
	c := Cache{
		Location:       testutils.Dir(t),
		Resolver:       resolver,
		Logger:         testutils.Logger(t, "mtasts"),
		downloadPolicy: mockDownloadPolicy(expectedPolicy, nil),
	}
	defer os.RemoveAll(c.Location)

	policy, err := c.Get(context.Background(), "example.org")
	if err != nil {
		t.Fatalf("policy get: %v", err)
	}
	if !reflect.DeepEqual(policy, expectedPolicy) {
		t.Fatalf("wrong policy returned, want %+v, got %+v", expectedPolicy, policy)
	}

	time.Sleep(2 * time.Second)

	// Don't let Refresh refetch the record.
	c.downloadPolicy = mockDownloadPolicy(nil, errors.New("broken"))

	if err := c.Refresh(); err != nil {
		t.Fatalf("cache refresh: %v", err)
	}

	// It should return the old record from cache.
	policy, err = c.Get(context.Background(), "example.org")
	if err != nil {
		t.Fatalf("policy get: %v", err)
	}
	if !reflect.DeepEqual(policy, expectedPolicy) {
		t.Fatalf("wrong policy returned, want %+v, got %+v", expectedPolicy, policy)
	}
}

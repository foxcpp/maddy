// The mtasts policy implements parsing, caching and checking of
// MTA-STS (RFC 8461) policies.
package mtasts

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/foxcpp/maddy/internal/dns"
)

type MalformedDNSRecordError struct {
	// Additional description of the error.
	Desc string
}

func (e MalformedDNSRecordError) Error() string {
	return fmt.Sprintf("mtasts: malformed DNS record: %s", e.Desc)
}

func readDNSRecord(raw string) (id string, err error) {
	parts := strings.Split(raw, ";")
	versionPresent := false
	for _, part := range parts {
		part = strings.TrimSpace(part)
		// handle k=v;k=v;
		//				 ^
		if part == "" {
			continue
		}
		kv := strings.Split(part, "=")
		if len(kv) != 2 {
			return "", MalformedDNSRecordError{Desc: "invalid record part: " + part}
		}

		if strings.ContainsAny(kv[0], " \t") || strings.ContainsAny(kv[1], " \t") {
			return "", MalformedDNSRecordError{Desc: "whitespace is not allowed in name or value"}
		}

		switch kv[0] {
		case "v":
			if kv[1] != "STSv1" {
				return "", MalformedDNSRecordError{Desc: "unsupported version: " + kv[1]}
			}
			versionPresent = true
		case "id":
			id = kv[1]
		}
	}
	if !versionPresent {
		return "", MalformedDNSRecordError{Desc: "missing version value"}
	}
	if id == "" {
		return "", MalformedDNSRecordError{Desc: "missing id value"}
	}
	return
}

type MalformedPolicyError struct {
	// Additional description of the error.
	Desc string
}

func (e MalformedPolicyError) Error() string {
	return fmt.Sprintf("mtasts: malformed policy: %s", e.Desc)
}

type Mode string

const (
	ModeEnforce Mode = "enforce"
	ModeTesting Mode = "testing"
	ModeNone    Mode = "none"
)

type Policy struct {
	Mode   Mode
	MaxAge int
	MX     []string
}

func readPolicy(contents io.Reader) (*Policy, error) {
	scnr := bufio.NewScanner(contents)
	policy := Policy{}

	present := make(map[string]struct{})

	for scnr.Scan() {
		fieldParts := strings.Split(scnr.Text(), ":")
		if len(fieldParts) != 2 {
			return nil, MalformedPolicyError{Desc: "invalid field: " + scnr.Text()}
		}

		// Arbitrary whitespace after colon:
		//	sts-policy-field-delim   = ":" *WSP
		fieldName := fieldParts[0]
		fieldValue := strings.TrimSpace(fieldParts[1])
		switch fieldName {
		case "version":
			if fieldValue != "STSv1" {
				return nil, MalformedPolicyError{Desc: "unsupported policy version: " + fieldValue}
			}
		case "mode":
			switch Mode(fieldValue) {
			case ModeEnforce, ModeTesting, ModeNone:
				policy.Mode = Mode(fieldValue)
			default:
				return nil, MalformedPolicyError{Desc: "invalid mode value: " + fieldValue}
			}
		case "max_age":
			var err error
			policy.MaxAge, err = strconv.Atoi(fieldValue)
			if err != nil {
				return nil, MalformedPolicyError{Desc: "invalid max_age value: " + err.Error()}
			}
		case "mx":
			policy.MX = append(policy.MX, fieldValue)
		}
		present[fieldName] = struct{}{}
	}
	if err := scnr.Err(); err != nil {
		return nil, err
	}

	if _, ok := present["version"]; !ok {
		return nil, MalformedPolicyError{Desc: "version field required"}
	}
	if _, ok := present["mode"]; !ok {
		return nil, MalformedPolicyError{Desc: "mode field required"}
	}
	if _, ok := present["max_age"]; !ok {
		return nil, MalformedPolicyError{Desc: "max_age field required"}
	}

	if policy.Mode != ModeNone && len(policy.MX) == 0 {
		return nil, MalformedPolicyError{Desc: "at least one mx field required when mode is not none"}
	}

	return &policy, nil
}

func (p Policy) Match(mx string) bool {
	normMX, err := dns.ForLookup(mx)
	if err != nil {
		return false
	}

	for _, pattern := range p.MX {
		normPattern, err := dns.ForLookup(pattern)
		if err != nil {
			continue
		}

		// Direct comparison is valid since both values are prepared using
		// dns.ForLookup.

		if strings.HasPrefix(normPattern, "*.") {
			firstDot := strings.Index(mx, ".")
			if firstDot == -1 {
				continue
			}

			if normMX[firstDot:] == normPattern[1:] {
				return true
			}
			continue
		}

		if normMX == normPattern {
			return true
		}
	}
	return false
}

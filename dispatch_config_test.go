package maddy

import (
	"reflect"
	"strings"
	"testing"

	"github.com/emersion/go-smtp"
	"github.com/foxcpp/maddy/config"
)

func policyError(code int) error {
	return &smtp.SMTPError{
		Message:      "Message rejected due to local policy",
		Code:         code,
		EnhancedCode: smtp.EnhancedCode{5, 7, 0},
	}
}

func TestDispatcherCfg(t *testing.T) {
	cases := []struct {
		name  string
		str   string
		value dispatcherCfg
		fail  bool
	}{
		{
			name: "basic",
			str: `
				source example.com {
					destination example.org {
						reject 410
				  	}
				  	default_destination {
						reject 420
				  	}
				}
				default_source {
				  	destination example.org {
						reject 430
				  	}
				  	default_destination {
						reject 440
				  	}
				}`,
			value: dispatcherCfg{
				perSource: map[string]sourceBlock{
					"example.com": {
						perRcpt: map[string]*rcptBlock{
							"example.org": {
								rejectErr: policyError(410),
							},
						},
						defaultRcpt: &rcptBlock{
							rejectErr: policyError(420),
						},
					},
				},
				defaultSource: sourceBlock{
					perRcpt: map[string]*rcptBlock{
						"example.org": {
							rejectErr: policyError(430),
						},
					},
					defaultRcpt: &rcptBlock{
						rejectErr: policyError(440),
					},
				},
			},
		},
		{
			name: "implied default destination",
			str: `
				source example.com {
					reject 410
				}
				default_source {
					reject 420
				}`,
			value: dispatcherCfg{
				perSource: map[string]sourceBlock{
					"example.com": {
						perRcpt: map[string]*rcptBlock{},
						defaultRcpt: &rcptBlock{
							rejectErr: policyError(410),
						},
					},
				},
				defaultSource: sourceBlock{
					perRcpt: map[string]*rcptBlock{},
					defaultRcpt: &rcptBlock{
						rejectErr: policyError(420),
					},
				},
			},
		},
		{
			name: "implied default sender",
			str: `
				destination example.com {
					reject 410
				}
				default_destination {
					reject 420
				}`,
			value: dispatcherCfg{
				perSource: map[string]sourceBlock{},
				defaultSource: sourceBlock{
					perRcpt: map[string]*rcptBlock{
						"example.com": {
							rejectErr: policyError(410),
						},
					},
					defaultRcpt: &rcptBlock{
						rejectErr: policyError(420),
					},
				},
			},
		},
		{
			name: "missing default source handler",
			str: `
				source example.org {
					reject 410
				}`,
			fail: true,
		},
		{
			name: "missing default destination handler",
			str: `
				destination example.org {
					reject 410
				}`,
			fail: true,
		},
		{
			name: "invalid domain",
			str: `
				destination example. {
					reject 410
				}
				default_destination {
					reject 500
				}`,
			fail: true,
		},
		{
			name: "invalid address",
			str: `
				destination @example. {
					reject 410
				}
				default_destination {
					reject 500
				}`,
			fail: true,
		},
		{
			name: "invalid address",
			str: `
				destination @example. {
					reject 421
				}
				default_destination {
					reject 500
				}`,
			fail: true,
		},
		{
			name: "invalid reject code",
			str: `
				destination example.com {
					reject 200
				}
				default_destination {
					reject 500
				}`,
			fail: true,
		},
		{
			name: "destination together with source",
			str: `
				destination example.com {
					reject 410
				}
				source example.org {
					reject 420
				}
				default_source {
					reject 430
				}`,
			fail: true,
		},
		{
			name: "empty destination rule",
			str: `
				destination {
					reject 410
				}
				default_destination {
					reject 420
				}`,
			fail: true,
		},
	}

	for _, case_ := range cases {
		case_ := case_
		t.Run(case_.name, func(t *testing.T) {
			cfg, _ := config.Read(strings.NewReader(case_.str), "literal")
			parsed, err := parseDispatcherRootCfg(nil, cfg)
			if err != nil && !case_.fail {
				t.Fatalf("unexpected parse error: %v", err)
			}
			if err == nil && case_.fail {
				t.Fatalf("unexpected parse success")
			}
			if case_.fail {
				t.Log(err)
				return
			}
			if !reflect.DeepEqual(parsed, case_.value) {
				t.Errorf("Wrong parsed configuration")
				t.Errorf("Wanted: %+v", case_.value)
				t.Errorf("Got: %+v", parsed)
			}
		})
	}
}

func TestDispatcherCfg_GlobalChecks(t *testing.T) {
	str := `
		check {
			test_check
		}
		default_destination {
			reject 500
		}
	`

	cfg, _ := config.Read(strings.NewReader(str), "literal")
	parsed, err := parseDispatcherRootCfg(nil, cfg)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	if len(parsed.globalChecks.checks) == 0 {
		t.Fatalf("missing test_check in globalChecks")
	}
}

func TestDispatcherCfg_SourceChecks(t *testing.T) {
	str := `
		source example.org {
			check {
				test_check
			}

			reject 500
		}
		default_source {
			reject 500
		}
	`

	cfg, _ := config.Read(strings.NewReader(str), "literal")
	parsed, err := parseDispatcherRootCfg(nil, cfg)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	if len(parsed.perSource["example.org"].checks.checks) == 0 {
		t.Fatalf("missing test_check in source checks")
	}
}

func TestDispatcherCfg_RcptChecks(t *testing.T) {
	str := `
		destination example.org {
			check {
				test_check
			}

			reject 500
		}
		default_destination {
			reject 500
		}
	`

	cfg, _ := config.Read(strings.NewReader(str), "literal")
	parsed, err := parseDispatcherRootCfg(nil, cfg)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	if len(parsed.defaultSource.perRcpt["example.org"].checks.checks) == 0 {
		t.Fatalf("missing test_check in rcpt checks")
	}
}

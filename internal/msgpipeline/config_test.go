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

import (
	"reflect"
	"strings"
	"testing"

	parser "github.com/foxcpp/maddy/framework/cfgparser"
	"github.com/foxcpp/maddy/framework/exterrors"
)

func policyError(code int) error {
	return &exterrors.SMTPError{
		Message:      "Message rejected due to a local policy",
		Code:         code,
		EnhancedCode: exterrors.EnhancedCode{5, 7, 0},
		Reason:       "reject directive used",
	}
}

func TestMsgPipelineCfg(t *testing.T) {
	cases := []struct {
		name  string
		str   string
		value msgpipelineCfg
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
			value: msgpipelineCfg{
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
			value: msgpipelineCfg{
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
			value: msgpipelineCfg{
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
				destination .. {
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
			cfg, _ := parser.Read(strings.NewReader(case_.str), "literal")
			parsed, err := parseMsgPipelineRootCfg(nil, cfg)
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

func TestMsgPipelineCfg_SourceIn(t *testing.T) {
	str := `
		source_in dummy {
			deliver_to dummy
		}
		default_source {
			reject 500
		}
	`

	cfg, _ := parser.Read(strings.NewReader(str), "literal")
	parsed, err := parseMsgPipelineRootCfg(nil, cfg)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	if len(parsed.sourceIn) == 0 {
		t.Fatalf("missing source_in dummy")
	}
}

func TestMsgPipelineCfg_DestIn(t *testing.T) {
	str := `
		destination_in dummy {
			deliver_to dummy
		}
		default_destination {
			reject 500
		}
	`

	cfg, _ := parser.Read(strings.NewReader(str), "literal")
	parsed, err := parseMsgPipelineRootCfg(nil, cfg)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	if len(parsed.defaultSource.rcptIn) == 0 {
		t.Fatalf("missing destination_in dummy")
	}
}

func TestMsgPipelineCfg_GlobalChecks(t *testing.T) {
	str := `
		check {
			test_check
		}
		default_destination {
			reject 500
		}
	`

	cfg, _ := parser.Read(strings.NewReader(str), "literal")
	parsed, err := parseMsgPipelineRootCfg(nil, cfg)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	if len(parsed.globalChecks) == 0 {
		t.Fatalf("missing test_check in globalChecks")
	}
}

func TestMsgPipelineCfg_GlobalChecksMultiple(t *testing.T) {
	str := `
		check {
			test_check
		}
		check {
			test_check
		}
		default_destination {
			reject 500
		}
	`

	cfg, _ := parser.Read(strings.NewReader(str), "literal")
	parsed, err := parseMsgPipelineRootCfg(nil, cfg)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	if len(parsed.globalChecks) != 2 {
		t.Fatalf("wrong amount of test_check's in globalChecks: %d", len(parsed.globalChecks))
	}
}

func TestMsgPipelineCfg_SourceChecks(t *testing.T) {
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

	cfg, _ := parser.Read(strings.NewReader(str), "literal")
	parsed, err := parseMsgPipelineRootCfg(nil, cfg)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	if len(parsed.perSource["example.org"].checks) == 0 {
		t.Fatalf("missing test_check in source checks")
	}
}

func TestMsgPipelineCfg_SourceChecks_Multiple(t *testing.T) {
	str := `
		source example.org {
			check {
				test_check
			}
			check {
				test_check
			}

			reject 500
		}
		default_source {
			reject 500
		}
	`

	cfg, _ := parser.Read(strings.NewReader(str), "literal")
	parsed, err := parseMsgPipelineRootCfg(nil, cfg)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	if len(parsed.perSource["example.org"].checks) != 2 {
		t.Fatalf("wrong amount of test_check's in source checks: %d", len(parsed.perSource["example.org"].checks))
	}
}

func TestMsgPipelineCfg_RcptChecks(t *testing.T) {
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

	cfg, _ := parser.Read(strings.NewReader(str), "literal")
	parsed, err := parseMsgPipelineRootCfg(nil, cfg)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	if len(parsed.defaultSource.perRcpt["example.org"].checks) == 0 {
		t.Fatalf("missing test_check in rcpt checks")
	}
}

func TestMsgPipelineCfg_RcptChecks_Multiple(t *testing.T) {
	str := `
		destination example.org {
			check {
				test_check
			}
			check {
				test_check
			}

			reject 500
		}
		default_destination {
			reject 500
		}
	`

	cfg, _ := parser.Read(strings.NewReader(str), "literal")
	parsed, err := parseMsgPipelineRootCfg(nil, cfg)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	if len(parsed.defaultSource.perRcpt["example.org"].checks) != 2 {
		t.Fatalf("wrong amount of test_check's in rcpt checks: %d", len(parsed.defaultSource.perRcpt["example.org"].checks))
	}
}

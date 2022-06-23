//go:build integration
// +build integration

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

package tests_test

import (
	"testing"

	"github.com/foxcpp/maddy/tests"
)

func TestReplaceAddr_Rcpt(tt *testing.T) {
	test := func(name, cfg string) {
		tt.Run(name, func(tt *testing.T) {
			t := tests.NewT(tt)
			t.DNS(nil)
			t.Port("smtp")
			t.Config(cfg)
			t.Run(1)
			defer t.Close()

			c := t.Conn("smtp")
			defer c.Close()
			c.SMTPNegotation("client.maddy.test", nil, nil)
			c.Writeln("MAIL FROM:<a@maddy.test>")
			c.ExpectPattern("250 *")
			c.Writeln("RCPT TO:<a@maddy.test>")
			c.ExpectPattern("250 *")
			c.Writeln("DATA")
			c.ExpectPattern("354 *")
			c.Writeln("From: <from@maddy.test>")
			c.Writeln("To: <to@maddy.test>")
			c.Writeln("Subject: Hello!")
			c.Writeln("")
			c.Writeln("Hello!")
			c.Writeln(".")
			c.ExpectPattern("250 2.0.0 OK: queued")
			c.Writeln("QUIT")
			c.ExpectPattern("221 *")
		})
	}

	test("inline", `
			hostname mx.maddy.test
			tls off

			smtp tcp://127.0.0.1:{env:TEST_PORT_smtp} {
				modify {
					replace_rcpt static {
						entry a@maddy.test b@maddy.test
					}
				}
				destination a@maddy.test {
					reject
				}
				destination b@maddy.test {
					deliver_to dummy
				}
				default_destination {
					reject
				}
			}`)
	test("inline qualified", `
			hostname mx.maddy.test
			tls off

			smtp tcp://127.0.0.1:{env:TEST_PORT_smtp} {
				modify {
					modify.replace_rcpt static {
						entry a@maddy.test b@maddy.test
					}
				}
				destination a@maddy.test {
					reject
				}
				destination b@maddy.test {
					deliver_to dummy
				}
				default_destination {
					reject
				}
			}`)

	// FIXME: Not implemented
	// test("external", `
	//		hostname mx.maddy.test
	//		tls off

	//		modify.replace_rcpt local_aliases {
	//			table static {
	//				entry a@maddy.test b@maddy.test
	//			}
	//		}

	//		smtp tcp://127.0.0.1:{env:TEST_PORT_smtp} {
	//			modify {
	//				&local_aliases
	//			}
	//			source a@maddy.test {
	//				destination a@maddy.test {
	//					reject
	//				}
	//				destination b@maddy.test {
	//					deliver_to dummy
	//				}
	//				default_destination {
	//					reject
	//				}
	//			}
	//			default_source {
	//				reject
	//			}
	//		}`)
}

func TestReplaceAddr_Sender(tt *testing.T) {
	test := func(name, cfg string) {
		tt.Run(name, func(tt *testing.T) {
			t := tests.NewT(tt)
			t.DNS(nil)
			t.Port("smtp")
			t.Config(cfg)
			t.Run(1)
			defer t.Close()

			c := t.Conn("smtp")
			defer c.Close()
			c.SMTPNegotation("client.maddy.test", nil, nil)
			c.Writeln("MAIL FROM:<a@maddy.test>")
			c.ExpectPattern("250 *")
			c.Writeln("RCPT TO:<a@maddy.test>")
			c.ExpectPattern("250 *")
			c.Writeln("DATA")
			c.ExpectPattern("354 *")
			c.Writeln("From: <from@maddy.test>")
			c.Writeln("To: <to@maddy.test>")
			c.Writeln("Subject: Hello!")
			c.Writeln("")
			c.Writeln("Hello!")
			c.Writeln(".")
			c.ExpectPattern("250 2.0.0 OK: queued")
			c.Writeln("QUIT")
			c.ExpectPattern("221 *")
		})
	}

	test("inline", `
			hostname mx.maddy.test
			tls off

			smtp tcp://127.0.0.1:{env:TEST_PORT_smtp} {
				modify {
					replace_sender static {
						entry a@maddy.test b@maddy.test
					}
				}
				source a@maddy.test {
					reject
				}
				source b@maddy.test {
					destination a@maddy.test {
						deliver_to dummy
					}
					default_destination {
						reject
					}
				}
				default_source {
					reject
				}
			}`)
	test("inline qualified", `
			hostname mx.maddy.test
			tls off

			smtp tcp://127.0.0.1:{env:TEST_PORT_smtp} {
				modify {
					modify.replace_sender static {
						entry a@maddy.test b@maddy.test
					}
				}
				source a@maddy.test {
					reject
				}
				source b@maddy.test {
					destination a@maddy.test {
						deliver_to dummy
					}
					default_destination {
						reject
					}
				}
				default_source {
					reject
				}
			}`)
	// FIXME: Not implemented
	// test("external", `
	//		hostname mx.maddy.test
	//		tls off

	//		modify.replace_sender local_aliases {
	//			table static {
	//				entry a@maddy.test b@maddy.test
	//			}
	//		}

	//		smtp tcp://127.0.0.1:{env:TEST_PORT_smtp} {
	//			modify {
	//				&local_aliases
	//			}
	//			source a@maddy.test {
	//				reject
	//			}
	//			source b@maddy.test {
	//				destination a@maddy.test {
	//					deliver_to dummy
	//				}
	//				default_destination {
	//					reject
	//				}
	//			}
	//			default_source {
	//				reject
	//			}
	//		}`)
}

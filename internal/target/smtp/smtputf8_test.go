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

package smtp_downstream

import (
	"testing"

	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/internal/testutils"
)

func TestDownstreamDelivery_EHLO_ALabel(t *testing.T) {
	be, srv := testutils.SMTPServer(t, "127.0.0.1:"+testPort)
	defer srv.Close()
	defer testutils.CheckSMTPConnLeak(t, srv)

	mod, err := NewDownstream("", "", nil, []string{"tcp://127.0.0.1:" + testPort})
	if err != nil {
		t.Fatal(err)
	}
	if err := mod.Init(config.NewMap(nil, config.Node{
		Children: []config.Node{
			{
				Name: "hostname",
				Args: []string{"тест.invalid"},
			},
		},
	})); err != nil {
		t.Fatal(err)
	}

	tgt := mod.(*Downstream)
	tgt.log = testutils.Logger(t, "remote")

	testutils.DoTestDelivery(t, tgt, "test@example.com", []string{"test@example.invalid"})

	be.CheckMsg(t, 0, "test@example.com", []string{"test@example.invalid"})
	if be.Messages[0].State.Hostname != "xn--e1aybc.invalid" {
		t.Error("target/remote should use use Punycode in EHLO")
	}
}

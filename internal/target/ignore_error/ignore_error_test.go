/*
Maddy Mail Server - Composable all-in-one email server.
Copyright © 2019-2025 Max Mazurov <fox.cpp@disroot.org>, Maddy Mail Server contributors

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

package ignore_error

import (
	"errors"
	"testing"

	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/internal/testutils"
)

func wrap(t *testing.T, inner *testutils.Target) *Target {
	return &Target{
		instName: "test",
		log:      testutils.Logger(t, modName),
		wrapped:  inner,
	}
}

func TestPassthroughOnSuccess(t *testing.T) {
	inner := &testutils.Target{}
	tgt := wrap(t, inner)

	testutils.DoTestDelivery(t, tgt, "from@example.org", []string{"to@example.org"})

	testutils.CheckTestMessage(t, inner, 0, "from@example.org", []string{"to@example.org"})
}

func TestIgnoredErrors(t *testing.T) {
	boom := errors.New("boom")

	cases := map[string]*testutils.Target{
		"StartDelivery": {StartErr: boom},
		"AddRcpt":       {RcptErr: map[string]error{"to@example.org": boom}},
		"Body":          {BodyErr: boom},
		"Commit":        {CommitErr: boom},
	}

	for name, inner := range cases {
		t.Run(name, func(t *testing.T) {
			tgt := wrap(t, inner)

			if _, err := testutils.DoTestDeliveryErr(t, tgt, "from@example.org", []string{"to@example.org"}); err != nil {
				t.Errorf("wrapped %s error was not ignored: %v", name, err)
			}
		})
	}
}

func TestConfigureRequiresTarget(t *testing.T) {
	tgt := &Target{instName: "test", log: testutils.Logger(t, modName)}

	if err := tgt.Configure(nil, config.NewMap(nil, config.Node{})); err == nil {
		t.Error("expected an error when no wrapped target is given")
	}
}

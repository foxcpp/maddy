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
	"fmt"

	"github.com/foxcpp/maddy/framework/module"
)

// objectName returns a new that is usable to identify the used external
// component (module or some stub) in debug logs.
func objectName(x interface{}) string {
	mod, ok := x.(module.Module)
	if ok {
		return mod.Name() + ":" + mod.InstanceName()
	}

	_, pipeline := x.(*MsgPipeline)
	if pipeline {
		return "reroute"
	}

	str, ok := x.(fmt.Stringer)
	if ok {
		return str.String()
	}

	return fmt.Sprintf("%T", x)
}

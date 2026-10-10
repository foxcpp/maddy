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

package target

import (
	"strings"

	"github.com/foxcpp/maddy/framework/log"
	"github.com/foxcpp/maddy/framework/module"
)

// DeliveryLogger returns a logger annotated with msg_id and from_domain so
// every log line can be filtered by message ID or sender domain.
func DeliveryLogger(parent *log.Logger, msgMeta *module.MsgMetadata) *log.Logger {
	kvpairs := []interface{}{"msg_id", msgMeta.ID}
	if msgMeta.OriginalFrom != "" {
		if _, domain, ok := strings.Cut(msgMeta.OriginalFrom, "@"); ok && domain != "" {
			kvpairs = append(kvpairs, "from_domain", domain)
		}
	}
	return parent.Sublogger("").With(kvpairs...)
}

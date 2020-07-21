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

package updatepipe

type BackendMode int

const (
	// ModeReplicate configures backend to both send and receive updates over
	// the pipe.
	ModeReplicate BackendMode = iota

	// ModePush configures backend to send updates over the pipe only.
	//
	// If EnableUpdatePipe(ModePush) is called for backend, its Updates()
	// channel will never receive any updates.
	ModePush BackendMode = iota
)

// The Backend interface is implemented by storage backends that support both
// updates serialization using the internal updatepipe.P implementation.
// To activate this implementation, EnableUpdatePipe should be called.
type Backend interface {
	// EnableUpdatePipe enables the internal update pipe implementation.
	// The mode argument selects the pipe behavior. EnableUpdatePipe must be
	// called before the first call to the Updates() method.
	//
	// This method is idempotent. All calls after a successful one do nothing.
	EnableUpdatePipe(mode BackendMode) error
}

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

package limiters

import "context"

// MultiLimit wraps multiple L implementations into a single one, locking them
// in the specified order.
//
// It does not implement any deadlock detection or avoidance algorithms.
type MultiLimit struct {
	Wrapped []L
}

func (ml *MultiLimit) Take() bool {
	for i := 0; i < len(ml.Wrapped); i++ {
		if !ml.Wrapped[i].Take() {
			// Acquire failed, undo acquire for all other resources we already
			// got.
			for _, l := range ml.Wrapped[:i] {
				l.Release()
			}
			return false
		}
	}
	return true
}

func (ml *MultiLimit) TakeContext(ctx context.Context) error {
	for i := 0; i < len(ml.Wrapped); i++ {
		if err := ml.Wrapped[i].TakeContext(ctx); err != nil {
			// Acquire failed, undo acquire for all other resources we already
			// got.
			for _, l := range ml.Wrapped[:i] {
				l.Release()
			}
			return err
		}
	}
	return nil
}

func (ml *MultiLimit) Release() {
	for _, l := range ml.Wrapped {
		l.Release()
	}
}

func (ml *MultiLimit) Close() {
	for _, l := range ml.Wrapped {
		l.Close()
	}
}

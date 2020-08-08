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

package lexer

import (
	"io"
)

// allTokens lexes the entire input, but does not parse it.
// It returns all the tokens from the input, unstructured
// and in order.
func allTokens(input io.Reader) ([]Token, error) {
	l := new(lexer)
	err := l.load(input)
	if err != nil {
		return nil, err
	}
	var tokens []Token
	for l.next() {
		tokens = append(tokens, l.token)
	}
	if err := l.err(); err != nil {
		return nil, err
	}
	return tokens, nil
}

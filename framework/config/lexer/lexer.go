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

// Copyright 2015 Light Code Labs, LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package lexer

import (
	"bufio"
	"io"
	"unicode"
)

type (
	// lexer is a utility which can get values, token by
	// token, from a Reader. A token is a word, and tokens
	// are separated by whitespace. A word can be enclosed
	// in quotes if it contains whitespace.
	lexer struct {
		reader *bufio.Reader
		token  Token
		line   int

		lastErr error
	}

	// Token represents a single parsable unit.
	Token struct {
		File string
		Line int
		Text string
	}
)

// load prepares the lexer to scan an input for tokens.
// It discards any leading byte order mark.
func (l *lexer) load(input io.Reader) error {
	l.reader = bufio.NewReader(input)
	l.line = 1

	// discard byte order mark, if present
	firstCh, _, err := l.reader.ReadRune()
	if err != nil {
		return err
	}
	if firstCh != 0xFEFF {
		err := l.reader.UnreadRune()
		if err != nil {
			return err
		}
	}

	return nil
}

func (l *lexer) err() error {
	return l.lastErr
}

// next loads the next token into the lexer.
//
// A token is delimited by whitespace, unless the token starts with a quotes
// character (") in which case the token goes until the closing quotes (the
// enclosing quotes are not included). Inside quoted strings, quotes may be
// escaped with a preceding \ character. No other chars may be escaped. Curly
// braces ('{', '}') are emitted as a separate tokens.
//
// The rest of the line is skipped if a "#" character is read in.
//
// Returns true if a token was loaded; false otherwise. If read from
// underlying Reader fails, next() returns false and err() will return the
// error occurred.
func (l *lexer) next() bool {
	var val []rune
	var comment, quoted, escaped bool

	makeToken := func() bool {
		l.token.Text = string(val)
		l.lastErr = nil
		return true
	}

	for {
		ch, _, err := l.reader.ReadRune()
		if err != nil {
			if len(val) > 0 {
				return makeToken()
			}
			if err == io.EOF {
				return false
			}
			l.lastErr = err
			return false
		}

		if quoted {
			if !escaped {
				if ch == '\\' {
					escaped = true
					continue
				} else if ch == '"' {
					return makeToken()
				}
			}
			if ch == '\n' {
				l.line++
			}
			if escaped {
				// only escape quotes
				if ch != '"' {
					val = append(val, '\\')
				}
			}
			val = append(val, ch)
			escaped = false
			continue
		}

		if unicode.IsSpace(ch) {
			if ch == '\r' {
				continue
			}
			if ch == '\n' {
				l.line++
				comment = false
			}
			if len(val) > 0 {
				return makeToken()
			}
			continue
		}

		if ch == '#' {
			comment = true
		}

		if comment {
			continue
		}

		if len(val) == 0 {
			l.token = Token{Line: l.line}
			if ch == '"' {
				quoted = true
				continue
			}
		}

		val = append(val, ch)
	}
}

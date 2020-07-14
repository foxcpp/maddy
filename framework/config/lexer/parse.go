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

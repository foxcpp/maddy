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

package exterrors

type fieldsErr interface {
	Fields() map[string]interface{}
}

type unwrapper interface {
	Unwrap() error
}

type fieldsWrap struct {
	err    error
	fields map[string]interface{}
}

func (fw fieldsWrap) Error() string {
	return fw.err.Error()
}

func (fw fieldsWrap) Unwrap() error {
	return fw.err
}

func (fw fieldsWrap) Fields() map[string]interface{} {
	return fw.fields
}

func Fields(err error) map[string]interface{} {
	fields := make(map[string]interface{}, 5)

	for err != nil {
		errFields, ok := err.(fieldsErr)
		if ok {
			for k, v := range errFields.Fields() {
				// Outer errors override fields of the inner ones.
				// Not the reverse.
				if fields[k] != nil {
					continue
				}
				fields[k] = v
			}
		}

		unwrap, ok := err.(unwrapper)
		if !ok {
			break
		}
		err = unwrap.Unwrap()
	}

	return fields
}

func WithFields(err error, fields map[string]interface{}) error {
	return fieldsWrap{err: err, fields: fields}
}

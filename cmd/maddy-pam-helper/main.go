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

package main

/*
#cgo LDFLAGS: -lpam
#cgo CFLAGS: -DCGO -Wall -Wextra -Werror -Wno-unused-parameter -Wno-error=unused-parameter -Wpedantic -std=c99
extern int run();
*/
import "C"
import "os"

/*
Apparently, some people would not want to build it manually by calling GCC.
Here we do it for them. Not going to tell them that resulting file is 800KiB
bigger than one built using only C compiler.
*/

func main() {
	i := int(C.run())
	os.Exit(i)
}

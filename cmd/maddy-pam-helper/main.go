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

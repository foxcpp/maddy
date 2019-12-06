package address

import (
	"strings"
	"testing"
)

func TestToASCII(t *testing.T) {
	test := addrFuncTest(t, ToASCII)
	test("test@тест.example.org", "test@xn--e1aybc.example.org", false)
	test("test@org."+strings.Repeat("x", 65535)+"\uFF00", "test@org."+strings.Repeat("x", 65535)+"\uFF00", true)
	test("тест@example.org", "тест@example.org", true)
	test("postmaster", "postmaster", false)
	test("postmaster@", "postmaster@", true)
}

func TestToUnicode(t *testing.T) {
	test := addrFuncTest(t, ToUnicode)
	test("test@xn--e1aybc.example.org", "test@тест.example.org", false)
	test("test@xn--9999999999999999999a.org", "test@xn--9999999999999999999a.org", true)
	test("postmaster", "postmaster", false)
	test("postmaster@", "postmaster@", true)
}

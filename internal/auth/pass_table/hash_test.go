package pass_table

import "testing"

func TestVerifyArgon2_Malformed(t *testing.T) {
	for _, hash := range []string{
		"",
		"1",
		"1:8:1",
		"1:8:1:U0FBQUFBTFQ=",
		"0:8:1:U0FBQUFBTFQ=:KHUshl3DcpHR3AoVd28ZeBGmZ1Fj1gwJgNn98Ia8DAvGHqI0BvFOMJPxtaAfO8F+qomm2O3h0P0yV50QGwXI/Q==",
		"1:8:0:U0FBQUFBTFQ=:KHUshl3DcpHR3AoVd28ZeBGmZ1Fj1gwJgNn98Ia8DAvGHqI0BvFOMJPxtaAfO8F+qomm2O3h0P0yV50QGwXI/Q==",
	} {
		if err := verifyArgon2("password", hash); err == nil {
			t.Errorf("verifyArgon2(%q) returned nil error", hash)
		}
	}
}

func TestVerifyArgon2(t *testing.T) {
	hash := "1:8:1:U0FBQUFBTFQ=:KHUshl3DcpHR3AoVd28ZeBGmZ1Fj1gwJgNn98Ia8DAvGHqI0BvFOMJPxtaAfO8F+qomm2O3h0P0yV50QGwXI/Q=="
	if err := verifyArgon2("password", hash); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := verifyArgon2("different-password", hash); err == nil {
		t.Error("expected mismatch error")
	}
}

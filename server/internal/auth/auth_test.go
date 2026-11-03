package auth

import (
	"strings"
	"testing"
)

func TestValidEmail(t *testing.T) {
	ok := []string{"a@b.co", "ust.az+tag@sub.example.id", strings.Repeat("a", 64) + "@" + strings.Repeat("b", 60) + ".com"}
	bad := []string{"", "a", "a@b", "@b.co", "a@.co", "a@b.", "a b@c.d", "a@b.c d", "Nama <a@b.co>", "a@b.co\n", "a@@b.co",
		"é@b.co", "a@b.co,c@d.co", "\x00@b.co", strings.Repeat("a", 65) + "@b.co", "a@" + strings.Repeat("b", 250) + ".co"}
	for _, s := range ok {
		if !validEmail(s) {
			t.Errorf("%q should be valid", s)
		}
	}
	for _, s := range bad {
		if validEmail(s) {
			t.Errorf("%q should be invalid", s)
		}
	}
}

func TestEmailLengthLimit(t *testing.T) {
	long := "a@" + strings.Repeat("b", 63) + "." + strings.Repeat("c", 63) + "." + strings.Repeat("d", 63) + "." + strings.Repeat("e", 56) + ".com"
	if len(long) != 254 || !validEmail(long) {
		t.Fatalf("254 bytes must pass (len %d)", len(long))
	}
	if validEmail("x" + long) {
		t.Fatal("255 bytes must fail")
	}
}

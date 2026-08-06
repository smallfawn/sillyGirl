package emoji

import "testing"

func TestHexToRuneRejectsInvalidCodePoints(t *testing.T) {
	for _, value := range []string{"110000", "D800", "FFFFFFFF", "invalid"} {
		if _, err := HexToRune(value); err == nil {
			t.Fatalf("HexToRune(%q) accepted an invalid code point", value)
		}
	}
	if got, err := HexToRune("1F600"); err != nil || got != '😀' {
		t.Fatalf("HexToRune valid result = %q, %v", got, err)
	}
}

func TestTransferRejectsMalformedOrNonSupplementaryInput(t *testing.T) {
	for _, value := range []string{"invalid", "FFFF", "110000"} {
		if got := transfer(value); len(got) != 0 {
			t.Fatalf("transfer(%q) = %#v", value, got)
		}
	}
	if got := transfer("1F600"); len(got) != 2 || got[0] != "D83D" || got[1] != "DE00" {
		t.Fatalf("transfer valid result = %#v", got)
	}
}

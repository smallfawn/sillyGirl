package utils

import "testing"

func TestContains(t *testing.T) {
	values := []string{"alpha", "beta"}
	if !Contains(values, "missing", "beta") {
		t.Fatal("Contains did not find a matching candidate")
	}
	if Contains(values, "missing") {
		t.Fatal("Contains returned true for a missing candidate")
	}
	if Contains(nil, "alpha") {
		t.Fatal("Contains returned true for an empty source")
	}
}

func TestUniqueSkipsNonStringInterfaceValues(t *testing.T) {
	got := Unique([]interface{}{"alpha", 123, nil, "alpha", "beta"})
	if len(got) != 2 || got[0] != "alpha" || got[1] != "beta" {
		t.Fatalf("Unique returned %#v", got)
	}
}

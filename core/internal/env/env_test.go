package env

import (
	"strings"
	"testing"
)

func TestExpandDoesNotRecursivelyAmplifyReferences(t *testing.T) {
	values := Var{
		"A": "${B}${B}",
		"B": "${A}${A}",
	}
	got := expand("${A}", values)
	if got != values["A"] {
		t.Fatalf("expand() = %q, want one-pass value %q", got, values["A"])
	}
	if len(got) > len(values["A"]) {
		t.Fatalf("expand recursively amplified output to %d bytes", len(got))
	}
}

func TestExpandLargeValueIsLinear(t *testing.T) {
	value := strings.Repeat("x", 1<<20)
	got := expand("prefix-${VALUE}-suffix", Var{"VALUE": value})
	if len(got) != len(value)+len("prefix--suffix") {
		t.Fatalf("unexpected expanded length: %d", len(got))
	}
}

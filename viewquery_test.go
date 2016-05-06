package main

import (
	"testing"
)

func assertSplit(t *testing.T, in string, exp []string) {
	out, err := parseQueryString(in)
	if err != nil {
		t.Fatalf("%q: %s", in, err)
	}

	if len(out) != len(exp) {
		t.Fatalf("expected %v but got %v", exp, out)
	}

	for i, v := range out {
		if exp[i] != v {
			t.Fatalf("[%d] expected %v but got %v", i, exp, out)
		}
	}
}

func TestQuerySplit(t *testing.T) {
	cases := map[string][]string{
		".a.b":                []string{"a", "b"},
		".a[5].b":             []string{"a", "[5]", "b"},
		".a[.name[0]=fish].b": []string{"a", "[.name[0]=fish]", "b"},
	}

	for in, exp := range cases {
		assertSplit(t, in, exp)
	}
}

func TestFindClosingBracket(t *testing.T) {
	if findClosingBracket("asdas]") != 5 {
		t.Fatal("expected 5")
	}

	if v := findClosingBracket("a[d]as]"); v != 6 {
		t.Fatal("expected 6, got ", v)
	}

	if v := findClosingBracket("a[[[[]]]]as]"); v != 11 {
		t.Fatal("expected 11, got ", v)
	}
}

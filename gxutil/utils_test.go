package gxutil

import (
	"testing"
)

func TestVersionComp(t *testing.T) {
	badreq, err := versionComp("1.2", "1.5.3")
	if err != nil {
		t.Fatal(err)
	}

	if !badreq {
		t.Fatal("should fail")
	}
}

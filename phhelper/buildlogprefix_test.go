package phhelper

import "testing"

func TestBuildLogPrefix_trimsAndDefaults(t *testing.T) {
	if p := BuildLogPrefix("  MyFunc  "); p != "[pchelper.MyFunc]" {
		t.Fatalf("got %q", p)
	}
	if p := BuildLogPrefix(""); p != "[pchelper.Log]" {
		t.Fatalf("empty name: got %q", p)
	}
}

package args

import (
	"reflect"
	"testing"
)

func TestParsePassthroughOnly(t *testing.T) {
	p, err := Parse([]string{"--dangerously-skip-permissions", "-p", "hi"})
	if err != nil {
		t.Fatal(err)
	}
	if p.SplitSet || p.List || p.NewSet {
		t.Fatalf("no wrapper flags expected, got %+v", p)
	}
	want := []string{"--dangerously-skip-permissions", "-p", "hi"}
	if !reflect.DeepEqual(p.Passthrough, want) {
		t.Fatalf("passthrough = %v, want %v", p.Passthrough, want)
	}
}

func TestParseSplitSpaceForm(t *testing.T) {
	p, err := Parse([]string{"--split", "work", "-p", "hi"})
	if err != nil {
		t.Fatal(err)
	}
	if !p.SplitSet || p.Split != "work" {
		t.Fatalf("Split=%q set=%v", p.Split, p.SplitSet)
	}
	if !reflect.DeepEqual(p.Passthrough, []string{"-p", "hi"}) {
		t.Fatalf("passthrough = %v", p.Passthrough)
	}
}

func TestParseSplitEqualsForm(t *testing.T) {
	p, _ := Parse([]string{"--split=work"})
	if !p.SplitSet || p.Split != "work" {
		t.Fatalf("Split=%q set=%v", p.Split, p.SplitSet)
	}
}

func TestParseBoolFlags(t *testing.T) {
	p, _ := Parse([]string{"--split-list"})
	if !p.List {
		t.Fatal("expected List=true")
	}
	if _, err := Parse([]string{"--split-list=x"}); err == nil {
		t.Fatal("expected error for value on bool flag")
	}
}

func TestParseValueFlagMissingValue(t *testing.T) {
	if _, err := Parse([]string{"--split"}); err == nil {
		t.Fatal("expected error for missing value")
	}
}

func TestParseAllValueFlags(t *testing.T) {
	p, _ := Parse([]string{"--split-new", "a"})
	if !p.NewSet || p.New != "a" {
		t.Fatalf("New=%q set=%v", p.New, p.NewSet)
	}
	p, _ = Parse([]string{"--split-default", "b"})
	if !p.DefaultSet || p.Default != "b" {
		t.Fatalf("Default=%q set=%v", p.Default, p.DefaultSet)
	}
	p, _ = Parse([]string{"--split-rm", "c"})
	if !p.RmSet || p.Rm != "c" {
		t.Fatalf("Rm=%q set=%v", p.Rm, p.RmSet)
	}
	p, _ = Parse([]string{"--split-which"})
	if !p.Which {
		t.Fatal("expected Which=true")
	}
}

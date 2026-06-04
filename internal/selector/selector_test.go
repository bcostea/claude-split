package selector

import (
	"bufio"
	"bytes"
	"strings"
	"testing"
)

func dec(s string) (key, error) {
	return decodeKey(bufio.NewReader(strings.NewReader(s)))
}

func TestDecodeKey(t *testing.T) {
	cases := []struct {
		in   string
		want key
	}{
		{"\r", keyEnter},
		{"\n", keyEnter},
		{"\x03", keyCancel},
		{"q", keyCancel},
		{"\x1b", keyCancel},      // bare ESC
		{"\x1bx", keyCancel},     // ESC not followed by [
		{"k", keyUp},
		{"j", keyDown},
		{"\x1b[A", keyUp},        // up arrow
		{"\x1b[B", keyDown},      // down arrow
		{"\x1b[C", keyNone},      // right arrow ignored
		{"z", keyNone},
	}
	for _, c := range cases {
		got, _ := dec(c.in)
		if got != c.want {
			t.Errorf("decodeKey(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func items2() []Item {
	return []Item{
		{Name: "default", Label: "default (home)"},
		{Name: "xogito", Label: "xogito", Status: "ok"},
	}
}

func TestRunDownEnterPicksSecond(t *testing.T) {
	in := strings.NewReader("\x1b[B\r") // down, enter
	var out bytes.Buffer
	res, err := Run(in, &out, items2(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Kind != Pick || res.Name != "xogito" {
		t.Fatalf("got %+v", res)
	}
}

func TestRunEnterImmediatelyPicksFirst(t *testing.T) {
	res, _ := Run(strings.NewReader("\r"), &bytes.Buffer{}, items2(), nil)
	if res.Kind != Pick || res.Name != "default" {
		t.Fatalf("got %+v", res)
	}
}

func TestRunNavigateToNewEntry(t *testing.T) {
	// two items -> "new" is index 2: down, down, enter
	in := strings.NewReader("\x1b[B\x1b[B\r")
	res, _ := Run(in, &bytes.Buffer{}, items2(), nil)
	if res.Kind != New {
		t.Fatalf("got %+v", res)
	}
}

func TestRunCtrlCCancels(t *testing.T) {
	res, _ := Run(strings.NewReader("\x03"), &bytes.Buffer{}, items2(), nil)
	if res.Kind != Cancel {
		t.Fatalf("got %+v", res)
	}
}

func TestRunUpClampsAtTop(t *testing.T) {
	// up at top stays on first, then enter
	res, _ := Run(strings.NewReader("\x1b[A\r"), &bytes.Buffer{}, items2(), nil)
	if res.Kind != Pick || res.Name != "default" {
		t.Fatalf("got %+v", res)
	}
}

func TestRunRendersItemsAndNewEntry(t *testing.T) {
	var out bytes.Buffer
	Run(strings.NewReader("\r"), &out, items2(), nil)
	s := out.String()
	for _, want := range []string{"default (home)", "xogito", "+ new split…"} {
		if !strings.Contains(s, want) {
			t.Fatalf("frame missing %q: %q", want, s)
		}
	}
}

func TestRunRendersFooter(t *testing.T) {
	var out bytes.Buffer
	footer := []string{"", "Commands: --split-list  --split-new <name>"}
	Run(strings.NewReader("\r"), &out, items2(), footer)
	if !strings.Contains(out.String(), "Commands: --split-list  --split-new <name>") {
		t.Fatalf("footer not rendered: %q", out.String())
	}
}

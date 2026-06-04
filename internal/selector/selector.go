// Package selector renders a minimal arrow-key menu for choosing a split.
// The caller is responsible for putting the terminal into raw mode before
// calling Run and restoring it afterwards; Run itself only reads bytes from
// in and writes frames to out, which keeps it testable without a real TTY.
package selector

import (
	"bufio"
	"fmt"
	"io"
)

// Kind is the outcome of a selector session.
type Kind int

const (
	Pick   Kind = iota // a menu item was chosen (see Name)
	New                // the "new split" entry was chosen
	Cancel             // the user aborted
)

// Result is what Run returns.
type Result struct {
	Kind Kind
	Name string // set when Kind == Pick ("default" for the home profile)
}

// Item is one selectable menu row.
type Item struct {
	Name   string // identifier returned on Pick
	Label  string // display text
	Status string // optional right-hand note (e.g. "ok", "no token")
}

type key int

const (
	keyNone key = iota
	keyUp
	keyDown
	keyEnter
	keyCancel
)

// decodeKey reads one logical keypress from r.
func decodeKey(r *bufio.Reader) (key, error) {
	b, err := r.ReadByte()
	if err != nil {
		return keyNone, err
	}
	switch b {
	case '\r', '\n':
		return keyEnter, nil
	case 0x03, 'q': // Ctrl-C or q
		return keyCancel, nil
	case 'k':
		return keyUp, nil
	case 'j':
		return keyDown, nil
	case 0x1b: // ESC — bare ESC cancels, ESC [ A/B are arrows
		next, err := r.ReadByte()
		if err != nil || next != '[' {
			return keyCancel, nil
		}
		dir, err := r.ReadByte()
		if err != nil {
			return keyNone, err
		}
		switch dir {
		case 'A':
			return keyUp, nil
		case 'B':
			return keyDown, nil
		default:
			return keyNone, nil
		}
	default:
		return keyNone, nil
	}
}

// Run shows items plus a trailing "new split" entry and returns the choice.
func Run(in io.Reader, out io.Writer, items []Item) (Result, error) {
	r := bufio.NewReader(in)
	n := len(items) + 1 // + the "new split" entry
	cursor := 0

	render(out, items, cursor, false)
	for {
		k, err := decodeKey(r)
		if err == io.EOF {
			return Result{Kind: Cancel}, nil
		}
		if err != nil {
			return Result{}, err
		}
		switch k {
		case keyUp:
			if cursor > 0 {
				cursor--
			}
		case keyDown:
			if cursor < n-1 {
				cursor++
			}
		case keyCancel:
			return Result{Kind: Cancel}, nil
		case keyEnter:
			if cursor == len(items) {
				return Result{Kind: New}, nil
			}
			return Result{Kind: Pick, Name: items[cursor].Name}, nil
		}
		render(out, items, cursor, true)
	}
}

// render draws the menu. When redraw is true it first moves the cursor up over
// the previous frame so the new frame overwrites it in place.
func render(out io.Writer, items []Item, cursor int, redraw bool) {
	n := len(items) + 1
	if redraw {
		fmt.Fprintf(out, "\x1b[%dA", n)
	}
	for i := 0; i <= len(items); i++ {
		marker := "  "
		if i == cursor {
			marker = "❯ "
		}
		var label string
		if i == len(items) {
			label = "+ new split…"
		} else {
			label = items[i].Label
			if items[i].Status != "" {
				label = fmt.Sprintf("%-14s%s", label, items[i].Status)
			}
		}
		line := marker + label
		if i == cursor {
			line = "\x1b[7m" + line + "\x1b[0m" // reverse video
		}
		fmt.Fprintf(out, "\r\x1b[K%s\n", line)
	}
}

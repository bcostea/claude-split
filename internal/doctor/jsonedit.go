package doctor

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

type member struct {
	key   string
	value json.RawMessage
}

// members decodes a top-level JSON object and keeps its key order, so an edit
// does not reorder the user's settings. Empty input is an empty object.
func members(raw []byte) ([]member, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, nil
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return nil, errors.New("top level is not a JSON object")
	}
	var out []member
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, ok := tok.(string)
		if !ok {
			return nil, fmt.Errorf("unexpected token %v", tok)
		}
		var v json.RawMessage
		if err := dec.Decode(&v); err != nil {
			return nil, err
		}
		out = append(out, member{key, v})
	}
	return out, nil
}

// stringList returns the string array stored at key, or nil if key is absent.
func stringList(raw []byte, key string) ([]string, error) {
	ms, err := members(raw)
	if err != nil {
		return nil, err
	}
	for _, m := range ms {
		if m.key == key {
			var list []string
			if err := json.Unmarshal(m.value, &list); err != nil {
				return nil, fmt.Errorf("%s: %w", key, err)
			}
			return list, nil
		}
	}
	return nil, nil
}

// setTopLevel sets key to value, in place if key exists, else at the end. The
// output uses two-space indent, as Claude Code writes settings.json.
func setTopLevel(raw []byte, key string, value any) ([]byte, error) {
	ms, err := members(raw)
	if err != nil {
		return nil, err
	}
	var enc bytes.Buffer
	e := json.NewEncoder(&enc)
	e.SetEscapeHTML(false)
	if err := e.Encode(value); err != nil {
		return nil, err
	}
	v := json.RawMessage(bytes.TrimSpace(enc.Bytes()))
	found := false
	for i := range ms {
		if ms[i].key == key {
			ms[i].value, found = v, true
		}
	}
	if !found {
		ms = append(ms, member{key, v})
	}

	var b bytes.Buffer
	b.WriteString("{")
	for i, m := range ms {
		if i > 0 {
			b.WriteString(",")
		}
		k, _ := json.Marshal(m.key)
		b.WriteString("\n  ")
		b.Write(k)
		b.WriteString(": ")
		if err := json.Indent(&b, m.value, "  ", "  "); err != nil {
			return nil, err
		}
	}
	if len(ms) > 0 {
		b.WriteString("\n")
	}
	b.WriteString("}\n")
	return b.Bytes(), nil
}

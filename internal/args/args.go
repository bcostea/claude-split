package args

import (
	"fmt"
	"strings"
)

// Parsed holds the wrapper-owned flags plus everything to forward to claude.
type Parsed struct {
	Split      string
	SplitSet   bool
	List       bool
	New        string
	NewSet     bool
	Default    string
	DefaultSet bool
	Rm         string
	RmSet      bool
	Which      bool
	Purge      bool

	Passthrough []string
}

var valueFlags = map[string]bool{
	"--split":         true,
	"--split-new":     true,
	"--split-default": true,
	"--split-rm":      true,
}

var boolFlags = map[string]bool{
	"--split-list":  true,
	"--split-which": true,
	"--split-purge": true,
}

// Parse partitions argv. Any argument that is not a recognized wrapper flag
// (and not the value consumed by one) is appended verbatim to Passthrough.
func Parse(argv []string) (Parsed, error) {
	var p Parsed
	for i := 0; i < len(argv); i++ {
		arg := argv[i]
		name, inlineVal, hasInline := arg, "", false
		if strings.HasPrefix(arg, "--split") {
			if eq := strings.IndexByte(arg, '='); eq >= 0 {
				name, inlineVal, hasInline = arg[:eq], arg[eq+1:], true
			}
		}

		switch {
		case valueFlags[name]:
			val := inlineVal
			if !hasInline {
				if i+1 >= len(argv) {
					return p, fmt.Errorf("flag %s requires a value", name)
				}
				i++
				val = argv[i]
			}
			switch name {
			case "--split":
				p.Split, p.SplitSet = val, true
			case "--split-new":
				p.New, p.NewSet = val, true
			case "--split-default":
				p.Default, p.DefaultSet = val, true
			case "--split-rm":
				p.Rm, p.RmSet = val, true
			}
		case boolFlags[name]:
			if hasInline {
				return p, fmt.Errorf("flag %s does not take a value", name)
			}
			switch name {
			case "--split-list":
				p.List = true
			case "--split-which":
				p.Which = true
			case "--split-purge":
				p.Purge = true
			}
		default:
			p.Passthrough = append(p.Passthrough, arg)
		}
	}
	return p, nil
}

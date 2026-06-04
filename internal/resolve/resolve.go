package resolve

// Action is what the launcher should do after resolution.
type Action int

const (
	LaunchDefault Action = iota // home profile, no CLAUDE_CONFIG_DIR
	LaunchSplit                 // a named split
	PrintListExit               // ambiguous — show the list and exit
)

type Decision struct {
	Action Action
	Split  string // set only when Action == LaunchSplit
}

// Resolve applies the order: explicit flag > configured default > prompt.
// When there are no splits at all and nothing chosen, the home profile is the
// only option, so launch it directly.
func Resolve(explicit string, explicitSet bool, configuredDefault string, splits []string) Decision {
	if explicitSet {
		if explicit == "default" {
			return Decision{Action: LaunchDefault}
		}
		return Decision{Action: LaunchSplit, Split: explicit}
	}
	if configuredDefault != "" {
		if configuredDefault == "default" {
			return Decision{Action: LaunchDefault}
		}
		return Decision{Action: LaunchSplit, Split: configuredDefault}
	}
	if len(splits) == 0 {
		return Decision{Action: LaunchDefault}
	}
	return Decision{Action: PrintListExit}
}

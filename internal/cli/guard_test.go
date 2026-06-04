package cli

import (
	"testing"

	"github.com/bcostea/claude-split/internal/resolve"
)

func TestHomeGuardExplicitSplitRefused(t *testing.T) {
	home := t.TempDir()
	d := resolve.Decision{Action: resolve.LaunchSplit, Split: "work"}
	_, _, err := applyHomeGuard(d, true /*explicit*/, home, home, false)
	if err == nil {
		t.Fatal("expected refusal for explicit split launched from home")
	}
}

func TestHomeGuardConfiguredDefaultDowngrades(t *testing.T) {
	home := t.TempDir()
	d := resolve.Decision{Action: resolve.LaunchSplit, Split: "work"}
	got, note, err := applyHomeGuard(d, false /*from configured default*/, home, home, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Action != resolve.LaunchDefault {
		t.Fatalf("expected downgrade to default, got %v", got.Action)
	}
	if note == "" {
		t.Fatal("expected a notice explaining the downgrade")
	}
}

func TestHomeGuardOutsideHomeUnaffected(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir() // a different directory
	d := resolve.Decision{Action: resolve.LaunchSplit, Split: "work"}
	got, note, err := applyHomeGuard(d, true, cwd, home, false)
	if err != nil || note != "" || got.Action != resolve.LaunchSplit {
		t.Fatalf("guard should not fire outside home: d=%v note=%q err=%v", got, note, err)
	}
}

func TestHomeGuardAllowOverride(t *testing.T) {
	home := t.TempDir()
	d := resolve.Decision{Action: resolve.LaunchSplit, Split: "work"}
	got, _, err := applyHomeGuard(d, true, home, home, true /*allow*/)
	if err != nil || got.Action != resolve.LaunchSplit {
		t.Fatalf("override should permit split in home: %v err=%v", got, err)
	}
}

func TestHomeGuardPromptBecomesDefaultInHome(t *testing.T) {
	home := t.TempDir()
	// No explicit split, no configured default, but splits exist → resolve
	// returns PrintListExit. In home this must become the default profile.
	d := resolve.Decision{Action: resolve.PrintListExit}
	got, _, err := applyHomeGuard(d, false, home, home, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Action != resolve.LaunchDefault {
		t.Fatalf("expected LaunchDefault in home, got %v", got.Action)
	}
}

func TestHomeGuardPromptOutsideHomeStillPrompts(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	d := resolve.Decision{Action: resolve.PrintListExit}
	got, _, err := applyHomeGuard(d, false, cwd, home, false)
	if err != nil || got.Action != resolve.PrintListExit {
		t.Fatalf("outside home the prompt must stand: %v err=%v", got, err)
	}
}

func TestHomeGuardDefaultActionUnaffected(t *testing.T) {
	home := t.TempDir()
	d := resolve.Decision{Action: resolve.LaunchDefault}
	got, _, err := applyHomeGuard(d, false, home, home, false)
	if err != nil || got.Action != resolve.LaunchDefault {
		t.Fatalf("default action in home is fine: %v err=%v", got, err)
	}
}

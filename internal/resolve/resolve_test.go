package resolve

import "testing"

func TestExplicitDefaultKeyword(t *testing.T) {
	d := Resolve("default", true, "work", []string{"work"})
	if d.Action != LaunchDefault {
		t.Fatalf("got %v", d.Action)
	}
}

func TestExplicitNamed(t *testing.T) {
	d := Resolve("work", true, "", []string{"work"})
	if d.Action != LaunchSplit || d.Split != "work" {
		t.Fatalf("got %+v", d)
	}
}

func TestFallsBackToConfiguredDefault(t *testing.T) {
	d := Resolve("", false, "work", []string{"work"})
	if d.Action != LaunchSplit || d.Split != "work" {
		t.Fatalf("got %+v", d)
	}
	d = Resolve("", false, "default", []string{"work"})
	if d.Action != LaunchDefault {
		t.Fatalf("got %+v", d)
	}
}

func TestNoDefaultWithSplitsPromptsList(t *testing.T) {
	d := Resolve("", false, "", []string{"work"})
	if d.Action != PrintListExit {
		t.Fatalf("got %v", d.Action)
	}
}

func TestNoDefaultNoSplitsUsesHome(t *testing.T) {
	d := Resolve("", false, "", nil)
	if d.Action != LaunchDefault {
		t.Fatalf("got %v", d.Action)
	}
}

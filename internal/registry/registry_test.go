package registry

import "testing"

func TestLoadMissingReturnsEmpty(t *testing.T) {
	r, err := Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Splits) != 0 || r.Default != "" {
		t.Fatalf("expected empty registry, got %+v", r)
	}
}

func TestAddSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	r, _ := Load(dir)
	if err := r.Add("work"); err != nil {
		t.Fatal(err)
	}
	if err := r.SetDefault("work"); err != nil {
		t.Fatal(err)
	}
	if err := r.Save(); err != nil {
		t.Fatal(err)
	}

	r2, _ := Load(dir)
	if !r2.Has("work") || r2.Default != "work" {
		t.Fatalf("round-trip lost data: %+v", r2)
	}
}

func TestAddDuplicateFails(t *testing.T) {
	r, _ := Load(t.TempDir())
	_ = r.Add("work")
	if err := r.Add("work"); err == nil {
		t.Fatal("expected duplicate error")
	}
}

func TestRemoveClearsDefaultAndRefusesDefault(t *testing.T) {
	r, _ := Load(t.TempDir())
	_ = r.Add("work")
	_ = r.SetDefault("work")
	if err := r.Remove("work"); err != nil {
		t.Fatal(err)
	}
	if r.Has("work") || r.Default != "" {
		t.Fatalf("remove failed: %+v", r)
	}
	if err := r.Remove("default"); err == nil {
		t.Fatal("expected refusal to remove 'default'")
	}
}

func TestSetDefaultUnknownFails(t *testing.T) {
	r, _ := Load(t.TempDir())
	if err := r.SetDefault("nope"); err == nil {
		t.Fatal("expected error for unknown split")
	}
	if err := r.SetDefault("default"); err != nil {
		t.Fatal("'default' should always be allowed")
	}
}

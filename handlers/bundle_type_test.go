package handlers

import "testing"

func TestNormalizeBundleType(t *testing.T) {
	cases := map[string]string{
		"corporate": "corporate",
		"standard":  "standard",
		"":          "standard",
		"garbage":   "standard",
		"CORPORATE": "standard", // case-sensitive on purpose
	}
	for in, want := range cases {
		if got := normalizeBundleType(in); got != want {
			t.Errorf("normalizeBundleType(%q) = %q, want %q", in, got, want)
		}
		if got := adminNormalizeBundleType(in); got != want {
			t.Errorf("adminNormalizeBundleType(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBuildAdminPermissions(t *testing.T) {
	// full admin => nil (unrestricted)
	if got := buildAdminPermissions("admin", []string{"users"}); got != nil {
		t.Errorf("expected nil for admin, got %v", *got)
	}
	// teacher => nil
	if got := buildAdminPermissions("teacher", []string{"users"}); got != nil {
		t.Errorf("expected nil for teacher, got %v", *got)
	}
	// coach with no perms => "[]" (deny by default, not nil)
	got := buildAdminPermissions("coach", nil)
	if got == nil || *got != "[]" {
		t.Errorf("expected \"[]\" for coach with no perms, got %v", got)
	}
	// coach with only invalid perms => "[]"
	got = buildAdminPermissions("coach", []string{"not-a-section"})
	if got == nil || *got != "[]" {
		t.Errorf("expected \"[]\" for coach with invalid perms, got %v", got)
	}
	// coach with valid perms => JSON array of the valid ones
	got = buildAdminPermissions("coach", []string{"users", "bogus", "payments"})
	if got == nil || *got != `["users","payments"]` {
		t.Errorf("got %v, want [\"users\",\"payments\"]", got)
	}
}

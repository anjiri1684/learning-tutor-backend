package middleware

import "testing"

func strptr(s string) *string { return &s }

func TestCoachHasSection(t *testing.T) {
	cases := []struct {
		name  string
		perms *string
		slug  string
		want  bool
	}{
		{"nil = no access", nil, "payments", false},
		{"empty string = no access", strptr(""), "payments", false},
		{"empty array = no access", strptr("[]"), "payments", false},
		{"literal null = no access", strptr("null"), "payments", false},
		{"malformed = no access", strptr("not json"), "payments", false},
		{"granted section", strptr(`["users","payments"]`), "payments", true},
		{"denied section", strptr(`["users","bookings"]`), "payments", false},
		{"single granted", strptr(`["requests"]`), "requests", true},
		{"single denied", strptr(`["requests"]`), "users", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := CoachHasSection(tc.perms, tc.slug); got != tc.want {
				t.Fatalf("CoachHasSection(%v, %q) = %v, want %v", tc.perms, tc.slug, got, tc.want)
			}
		})
	}
}

func TestAdminSectionsCanonical(t *testing.T) {
	required := []string{"users", "bookings", "bundles", "corporate-enquiries", "requests", "payments"}
	for _, r := range required {
		found := false
		for _, s := range AdminSections {
			if s == r {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("required section %q missing from AdminSections", r)
		}
	}
	for _, s := range AdminSections {
		if s == "settings" {
			t.Error("\"settings\" must not be in AdminSections (it is each admin's own profile)")
		}
	}
}

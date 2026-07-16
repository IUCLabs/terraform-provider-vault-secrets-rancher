package provider

import (
	"encoding/json"
	"testing"
)

func TestToInt64(t *testing.T) {
	cases := []struct {
		in   interface{}
		want int64
		ok   bool
	}{
		{json.Number("42"), 42, true},
		{json.Number("3.0"), 3, true},
		{float64(7), 7, true},
		{int(9), 9, true},
		{int64(11), 11, true},
		{"nope", 0, false},
		{nil, 0, false},
	}
	for _, c := range cases {
		got, ok := toInt64(c.in)
		if ok != c.ok || got != c.want {
			t.Errorf("toInt64(%v) = (%d,%v), want (%d,%v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestSplitBackendName(t *testing.T) {
	cases := []struct {
		id, marker, backend, name string
		ok                        bool
	}{
		{"rancher/roles/ci", "roles", "rancher", "ci", true},
		{"team/rancher/service-accounts/admin", "service-accounts", "team/rancher", "admin", true},
		{"rancher/roles/", "roles", "", "", false},
		{"/roles/ci", "roles", "", "", false},
		{"nomarker", "roles", "", "", false},
	}
	for _, c := range cases {
		b, n, ok := splitBackendName(c.id, c.marker)
		if ok != c.ok || b != c.backend || n != c.name {
			t.Errorf("splitBackendName(%q,%q) = (%q,%q,%v), want (%q,%q,%v)",
				c.id, c.marker, b, n, ok, c.backend, c.name, c.ok)
		}
	}
}

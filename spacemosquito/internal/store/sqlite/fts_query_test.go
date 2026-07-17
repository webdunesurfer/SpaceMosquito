package sqlite

import "testing"

func TestBuildFTSQuery_AND(t *testing.T) {
	tests := []struct {
		query string
		want  string
	}{
		{"foo", `"foo"`},
		{"Foo Bar Baz", `"Foo" AND "Bar" AND "Baz"`},
		{"  alpha   beta  ", `"alpha" AND "beta"`},
	}
	for _, tc := range tests {
		if got := buildFTSQuery(tc.query); got != tc.want {
			t.Errorf("buildFTSQuery(%q) = %q, want %q", tc.query, got, tc.want)
		}
	}
}

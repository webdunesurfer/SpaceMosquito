package cliapp

import "testing"

func TestParseSearchArgs(t *testing.T) {
	tests := []struct {
		args     []string
		wantQ    string
		wantSpace string
		wantLimit int
		wantErr  bool
	}{
		{[]string{"mosquito"}, "mosquito", "", 10, false},
		{[]string{"alpha beta", "TST"}, "alpha beta", "TST", 10, false},
		{[]string{"alpha beta", "--limit", "50"}, "alpha beta", "", 50, false},
		{[]string{"alpha beta", "TST", "--limit", "25"}, "alpha beta", "TST", 25, false},
		{[]string{"--limit"}, "", "", 0, true},
		{[]string{"--limit", "0"}, "", "", 0, true},
	}
	for _, tc := range tests {
		q, space, limit, err := parseSearchArgs(tc.args)
		if tc.wantErr {
			if err == nil {
				t.Errorf("parseSearchArgs(%v): want error", tc.args)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseSearchArgs(%v): %v", tc.args, err)
			continue
		}
		if q != tc.wantQ || space != tc.wantSpace || limit != tc.wantLimit {
			t.Errorf("parseSearchArgs(%v) = (%q, %q, %d), want (%q, %q, %d)",
				tc.args, q, space, limit, tc.wantQ, tc.wantSpace, tc.wantLimit)
		}
	}
}

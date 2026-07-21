package csf

import "testing"

func TestIsStorageFormat(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"structured-macro", `<ac:structured-macro ac:name="code"/>`, true},
		{"ri tag", `<p><ri:page ri:content-title="X"/></p>`, true},
		{"xmlns declaration", `<div xmlns:ac="http://x"><p>hi</p></div>`, true},
		{"rendered html", `<div class="code panel"><pre>x</pre></div>`, false},
		{"plain paragraph", `<p>just text</p>`, false},
		{"empty", ``, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsStorageFormat(tc.in); got != tc.want {
				t.Fatalf("IsStorageFormat(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

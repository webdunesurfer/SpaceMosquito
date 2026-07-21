package csf

import (
	"strings"
	"testing"
)

func TestEmoticon_emojiFallbackPreferred(t *testing.T) {
	got := md(t, `<p>done <ac:emoticon ac:name="blue-star" ac:emoji-fallback="👍"/> ok</p>`)
	if !strings.Contains(got, "👍") {
		t.Fatalf("emoji-fallback not used: %q", got)
	}
}

func TestEmoticon_legacyStaticMap(t *testing.T) {
	got := md(t, `<p>status <ac:emoticon ac:name="tick"/> and <ac:emoticon ac:name="cross"/></p>`)
	if !strings.Contains(got, "✓") || !strings.Contains(got, "✗") {
		t.Fatalf("legacy emoticon map failed: %q", got)
	}
}

func TestEmoticon_unknownDropped(t *testing.T) {
	got := md(t, `<p>before <ac:emoticon ac:name="totally-unknown-thing"/> after</p>`)
	if strings.Contains(got, "totally-unknown-thing") {
		t.Fatalf("unknown emoticon name leaked: %q", got)
	}
	if !strings.Contains(got, "before") || !strings.Contains(got, "after") {
		t.Fatalf("surrounding text lost: %q", got)
	}
}

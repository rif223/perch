package player

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseSource(t *testing.T) {
	file := filepath.Join(t.TempDir(), "clip.mp4")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	valid := map[string]string{
		"https://www.youtube.com/watch?v=jNQXAC9IVRw": KindYouTube,
		"youtu.be/jNQXAC9IVRw":                        KindYouTube,
		" https://m.twitch.tv/lofigirl ":              KindTwitch,
		"https://example.com/live/index.m3u8?t=1":     KindHLS,
		"https://example.com/manifest.mpd":            KindDASH,
		"http://example.com/video.mp4":                KindDirect,
		"rtmp://live.example.com/app/key":             KindDirect,
		`"` + file + `"`:                              KindFile,
	}
	for in, kind := range valid {
		src, err := ParseSource(in)
		if err != nil {
			t.Errorf("%q: unexpected error %v", in, err)
			continue
		}
		if src.Kind != kind {
			t.Errorf("%q: kind %q, want %q", in, src.Kind, kind)
		}
	}

	for _, in := range []string{
		"", "   ", "hallo", "javascript:alert(1)", "ftp://example.com/a.mp4",
		"https://", "https://intranet/video", "https://example.com/\nx",
		filepath.Join(t.TempDir(), "missing.mp4"), t.TempDir(),
	} {
		if _, err := ParseSource(in); err == nil {
			t.Errorf("%q: expected error", in)
		}
	}
}

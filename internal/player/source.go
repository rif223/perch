package player

import (
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// Source kinds reported to the UI.
const (
	KindYouTube = "youtube"
	KindTwitch  = "twitch"
	KindHLS     = "hls"
	KindDASH    = "dash"
	KindDirect  = "direct"
	KindFile    = "file"
)

// Source is a validated, normalized media location.
type Source struct {
	URL  string `json:"url"`
	Kind string `json:"kind"`
}

// NeedsYTDLP reports whether mpv resolves this source through yt-dlp.
func (s Source) NeedsYTDLP() bool {
	return s.Kind == KindYouTube || s.Kind == KindTwitch
}

var allowedSchemes = map[string]bool{
	"http": true, "https": true,
	"rtmp": true, "rtmps": true, "rtsp": true, "rtsps": true,
	"srt": true, "mms": true, "mmsh": true,
}

// ParseSource validates user input. It accepts network URLs (a missing
// https:// is added), file:// URLs and paths to existing local files.
func ParseSource(raw string) (Source, error) {
	raw = strings.Trim(strings.TrimSpace(raw), `"'`)
	if raw == "" {
		return Source{}, errors.New("Please enter a video or stream address.")
	}
	if len(raw) > 4096 {
		return Source{}, errors.New("The address is too long.")
	}
	for _, r := range raw {
		if unicode.IsControl(r) {
			return Source{}, errors.New("The address contains invalid characters.")
		}
	}

	if src, ok, err := parseLocalFile(raw); ok || err != nil {
		return src, err
	}

	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.Hostname() == "" {
		return Source{}, errors.New("This is not a valid address. Example: https://www.youtube.com/watch?v=…")
	}
	scheme := strings.ToLower(u.Scheme)
	if !allowedSchemes[scheme] {
		return Source{}, errors.New("Unsupported protocol \"" + u.Scheme + "\".")
	}
	host := strings.ToLower(u.Hostname())
	if !strings.Contains(host, ".") && host != "localhost" && !strings.Contains(host, ":") {
		return Source{}, errors.New("The address does not contain a valid host name.")
	}

	return Source{URL: u.String(), Kind: classify(host, u.Path)}, nil
}

func parseLocalFile(raw string) (Source, bool, error) {
	path := raw
	if strings.HasPrefix(strings.ToLower(raw), "file://") {
		u, err := url.Parse(raw)
		if err != nil {
			return Source{}, true, errors.New("Invalid file address.")
		}
		path = filepath.FromSlash(strings.TrimPrefix(u.Path, "/"))
	} else if !filepath.IsAbs(raw) {
		return Source{}, false, nil
	}
	st, err := os.Stat(path)
	if err != nil {
		return Source{}, true, errors.New("File not found: " + path)
	}
	if st.IsDir() {
		return Source{}, true, errors.New("Please choose a file, not a folder.")
	}
	return Source{URL: path, Kind: KindFile}, true, nil
}

func classify(host, path string) string {
	is := func(domains ...string) bool {
		for _, d := range domains {
			if host == d || strings.HasSuffix(host, "."+d) {
				return true
			}
		}
		return false
	}
	path = strings.ToLower(path)
	switch {
	case is("youtube.com", "youtu.be", "youtube-nocookie.com"):
		return KindYouTube
	case is("twitch.tv"):
		return KindTwitch
	case strings.HasSuffix(path, ".m3u8"):
		return KindHLS
	case strings.HasSuffix(path, ".mpd"):
		return KindDASH
	}
	return KindDirect
}

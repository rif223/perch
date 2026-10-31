package player

import (
	"os"
	"os/exec"
	"path/filepath"
)

// Binaries holds the resolved tool paths. An empty YTDLP means mpv falls back
// to its own PATH lookup.
type Binaries struct {
	MPV   string `json:"mpv"`
	YTDLP string `json:"ytdlp"`
}

// LocateBinaries checks, in order: environment override, next to the
// Perch EXE, common install folders (including WinGet), PATH.
func LocateBinaries() Binaries {
	exeDir := ""
	if exe, err := os.Executable(); err == nil {
		exeDir = filepath.Dir(exe)
	}
	local := os.Getenv("LOCALAPPDATA")
	programFiles := os.Getenv("ProgramFiles")
	winget := join(local, "Microsoft", "WinGet")

	mpv := firstExisting(
		os.Getenv("PERCH_MPV"),
		join(exeDir, "mpv.exe"),
		join(exeDir, "mpv", "mpv.exe"),
		join(programFiles, "MPV Player", "mpv.exe"),
		join(programFiles, "mpv", "mpv.exe"),
		join(local, "Programs", "mpv", "mpv.exe"),
		join(winget, "Links", "mpv.exe"),
		firstMatch(join(winget, "Packages", "shinchiro.mpv_*", "mpv.exe")),
		firstMatch(join(winget, "Packages", "shinchiro.mpv_*", "*", "mpv.exe")),
	)
	if mpv == "" {
		mpv, _ = exec.LookPath("mpv.exe")
	}

	ytdlp := firstExisting(
		os.Getenv("PERCH_YTDLP"),
		join(exeDir, "yt-dlp.exe"),
		join(winget, "Links", "yt-dlp.exe"),
		firstMatch(join(winget, "Packages", "yt-dlp.yt-dlp_*", "yt-dlp.exe")),
	)
	if ytdlp == "" {
		ytdlp, _ = exec.LookPath("yt-dlp.exe")
	}
	return Binaries{MPV: mpv, YTDLP: ytdlp}
}

func join(base string, elem ...string) string {
	if base == "" {
		return ""
	}
	return filepath.Join(append([]string{base}, elem...)...)
}

// firstMatch returns the first file matching a glob pattern, or "".
func firstMatch(pattern string) string {
	if pattern == "" {
		return ""
	}
	if m, _ := filepath.Glob(pattern); len(m) > 0 {
		return m[0]
	}
	return ""
}

func firstExisting(paths ...string) string {
	for _, p := range paths {
		if p == "" {
			continue
		}
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

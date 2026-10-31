package player

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

var updateMu sync.Mutex

// wingetNoUpdate is APPINSTALLER_CLI_ERROR_UPDATE_NOT_APPLICABLE.
const wingetNoUpdate = 0x8A15002B

// hiddenCommand runs a console program without flashing a console window.
func hiddenCommand(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000} // CREATE_NO_WINDOW
	return cmd
}

// YTDLPVersion returns the installed yt-dlp version, or "" if unknown.
func (m *Manager) YTDLPVersion() string {
	if m.bins.YTDLP == "" {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	out, err := hiddenCommand(ctx, m.bins.YTDLP, "--version").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// UpdateYTDLP updates yt-dlp through WinGet when it was installed that way
// (so WinGet keeps track of the version), otherwise with yt-dlp's own updater.
func (m *Manager) UpdateYTDLP() (string, error) {
	if m.bins.YTDLP == "" {
		return "", errors.New("yt-dlp was not found.")
	}
	updateMu.Lock()
	defer updateMu.Unlock()

	before := m.YTDLPVersion()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	cmd := hiddenCommand(ctx, m.bins.YTDLP, "-U")
	if strings.Contains(strings.ToLower(m.bins.YTDLP), `\microsoft\winget\`) {
		if winget, err := exec.LookPath("winget.exe"); err == nil {
			cmd = hiddenCommand(ctx, winget, "upgrade", "--id", "yt-dlp.yt-dlp", "--exact",
				"--silent", "--disable-interactivity",
				"--accept-source-agreements", "--accept-package-agreements")
		}
	}
	out, runErr := cmd.CombinedOutput()
	after := m.YTDLPVersion()

	// WinGet's messages are localized, so its "no update available" exit
	// code is checked instead of the text.
	var exitErr *exec.ExitError
	noUpdate := errors.As(runErr, &exitErr) && uint32(exitErr.ExitCode()) == wingetNoUpdate
	switch {
	case after != "" && after != before:
		return fmt.Sprintf("yt-dlp updated: %s → %s", before, after), nil
	case after != "" && (runErr == nil || noUpdate):
		return fmt.Sprintf("yt-dlp is already up to date (%s).", after), nil
	case ctx.Err() != nil:
		return "", errors.New("The yt-dlp update took too long and was cancelled.")
	}
	return "", fmt.Errorf("The yt-dlp update failed: %s", lastLine(string(out)))
}

func lastLine(s string) string {
	lines := strings.Split(strings.ReplaceAll(strings.TrimSpace(s), "\r", "\n"), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if l := strings.TrimSpace(lines[i]); l != "" {
			return l
		}
	}
	return "unknown error"
}

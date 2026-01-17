package clipboard

import (
	"os/exec"
	"runtime"
	"strings"
)

// Copy copies text to the system clipboard
func Copy(text string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "linux":
		// Try wl-copy first (Wayland), then xclip (X11)
		if _, err := exec.LookPath("wl-copy"); err == nil {
			cmd = exec.Command("wl-copy")
		} else if _, err := exec.LookPath("xclip"); err == nil {
			cmd = exec.Command("xclip", "-selection", "clipboard")
		} else if _, err := exec.LookPath("xsel"); err == nil {
			cmd = exec.Command("xsel", "--clipboard", "--input")
		} else {
			// Fallback: try xclip anyway
			cmd = exec.Command("xclip", "-selection", "clipboard")
		}
	default:
		// Windows or other - try clip.exe
		cmd = exec.Command("clip")
	}

	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

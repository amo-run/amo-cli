package env

import (
	"bytes"
	"errors"
	"os/exec"
	"runtime"
	"strings"
)

// Clipboard provides cross-platform clipboard text operations.
type Clipboard struct{}

// NewClipboard creates a new clipboard helper.
func NewClipboard() *Clipboard { return &Clipboard{} }

// ReadText reads plain text from the system clipboard.
func (c *Clipboard) ReadText() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		// macOS: use pbpaste
		cmd := exec.Command("pbpaste")
		out, err := cmd.Output()
		if err != nil {
			return "", err
		}
		return string(out), nil

	case "windows":
		// Prefer PowerShell Get-Clipboard
		if psPath, err := exec.LookPath("powershell"); err == nil {
			// -Raw to avoid line splitting; -NoProfile/-NonInteractive for speed/stability
			cmd := exec.Command(psPath, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", "Get-Clipboard -Raw")
			out, err := cmd.Output()
			if err == nil {
				return string(out), nil
			}
			// Fallback to Windows.Forms Clipboard (older environments). Requires STA.
			fallback := "Add-Type -AssemblyName System.Windows.Forms; [System.Windows.Forms.Clipboard]::GetText()"
			cmd = exec.Command(psPath, "-NoProfile", "-NonInteractive", "-STA", "-ExecutionPolicy", "Bypass", "-Command", fallback)
			out, err = cmd.Output()
			if err == nil {
				return string(out), nil
			}
		}
		return "", errors.New("no supported clipboard read method found (requires PowerShell)")

	default:
		// Linux/BSD: try Wayland first, then X11 tools
		if path, err := exec.LookPath("wl-paste"); err == nil {
			cmd := exec.Command(path)
			out, err := cmd.Output()
			if err == nil {
				return string(out), nil
			}
		}
		if path, err := exec.LookPath("xclip"); err == nil {
			cmd := exec.Command(path, "-selection", "clipboard", "-o")
			out, err := cmd.Output()
			if err == nil {
				return string(out), nil
			}
		}
		if path, err := exec.LookPath("xsel"); err == nil {
			cmd := exec.Command(path, "--clipboard", "--output")
			out, err := cmd.Output()
			if err == nil {
				return string(out), nil
			}
		}
		return "", errors.New("no clipboard utility found (install wl-clipboard, xclip, or xsel)")
	}
}

// WriteText writes plain text to the system clipboard.
func (c *Clipboard) WriteText(text string) error {
	data := []byte(text)
	switch runtime.GOOS {
	case "darwin":
		cmd := exec.Command("pbcopy")
		cmd.Stdin = bytes.NewReader(data)
		return cmd.Run()

	case "windows":
		// Prefer PowerShell Set-Clipboard reading from STDIN to avoid quoting issues
		if psPath, err := exec.LookPath("powershell"); err == nil {
			cmd := exec.Command(psPath, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", "Set-Clipboard -Value (Get-Content -Raw -)")
			cmd.Stdin = bytes.NewReader(data)
			if err := cmd.Run(); err == nil {
				return nil
			}
		}
		// Fallback to clip.exe via cmd; ensure trailing newline so clip captures correctly
		cmd := exec.Command("cmd", "/c", "clip")
		stdin := bytes.NewBuffer(data)
		if !strings.HasSuffix(text, "\n") {
			stdin.WriteByte('\n')
		}
		cmd.Stdin = stdin
		return cmd.Run()

	default:
		if path, err := exec.LookPath("wl-copy"); err == nil {
			cmd := exec.Command(path)
			cmd.Stdin = bytes.NewReader(data)
			if err := cmd.Run(); err == nil {
				return nil
			}
		}
		if path, err := exec.LookPath("xclip"); err == nil {
			cmd := exec.Command(path, "-selection", "clipboard")
			cmd.Stdin = bytes.NewReader(data)
			if err := cmd.Run(); err == nil {
				return nil
			}
		}
		if path, err := exec.LookPath("xsel"); err == nil {
			cmd := exec.Command(path, "--clipboard", "--input")
			cmd.Stdin = bytes.NewReader(data)
			if err := cmd.Run(); err == nil {
				return nil
			}
		}
		return errors.New("no clipboard utility found (install wl-clipboard, xclip, or xsel)")
	}
}

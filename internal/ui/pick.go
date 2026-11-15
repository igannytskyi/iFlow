package ui

import (
	"context"
	"errors"
	"os/exec"
	"runtime"
	"strings"
)

// errNoPicker says the platform offers no folder chooser this program can
// call, so a path has to be typed.
var errNoPicker = errors.New("no folder chooser")

// pickFolder asks the system to let a person choose a folder, and returns
// its path, or "" when they cancelled.
func pickFolder(ctx context.Context, prompt string) (string, error) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.CommandContext(ctx, "osascript",
			"-e", "activate",
			"-e", `POSIX path of (choose folder with prompt "`+strings.ReplaceAll(prompt, `"`, `'`)+`")`)
	case "linux":
		bin, err := exec.LookPath("zenity")
		if err != nil {
			return "", errNoPicker
		}
		cmd = exec.CommandContext(ctx, bin, "--file-selection", "--directory", "--title="+prompt)
	case "windows":
		cmd = exec.CommandContext(ctx, "powershell", "-NoProfile", "-Command",
			`Add-Type -AssemblyName System.Windows.Forms; $d = New-Object System.Windows.Forms.FolderBrowserDialog; `+
				`$d.Description = '`+strings.ReplaceAll(prompt, "'", "")+`'; if ($d.ShowDialog() -eq 'OK') { $d.SelectedPath }`)
	default:
		return "", errNoPicker
	}
	out, err := cmd.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return "", nil // cancelled
		}
		return "", errNoPicker
	}
	return strings.TrimRight(strings.TrimSpace(string(out)), "/\\"), nil
}

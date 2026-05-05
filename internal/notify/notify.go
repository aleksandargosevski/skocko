package notify

import (
	"os/exec"
	"runtime"
)

// Send sends a desktop notification. Silent no-op if not supported.
func Send(title, body string) {
	switch runtime.GOOS {
	case "darwin":
		_ = exec.Command("osascript", "-e",
			`display notification "`+body+`" with title "`+title+`"`).Run()
	case "linux":
		if path, err := exec.LookPath("notify-send"); err == nil {
			_ = exec.Command(path, title, body).Run()
		}
	}
}

package activator

import (
	"os/exec"
)

func checkoutHead(path string) error {
	cmd := exec.Command("cd", path+";", "git", "checkout", "HEAD")
	return cmd.Run()
}

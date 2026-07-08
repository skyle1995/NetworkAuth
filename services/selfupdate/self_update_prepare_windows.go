//go:build windows

package selfupdate

import (
	"os/exec"
)

// selfUpdateSetProcAttr 为 Windows 平台设置进程属性
// Windows 下不需要设置 Setpgid
func selfUpdateSetProcAttr(cmd *exec.Cmd) {
	// no-op on Windows
}

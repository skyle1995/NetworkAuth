//go:build !windows

package selfupdate

import (
	"os/exec"
	"syscall"
)

// selfUpdateSetProcAttr 为 Unix 平台设置进程属性
// 将新进程放入独立进程组，使其脱离父进程生命周期
func selfUpdateSetProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
}

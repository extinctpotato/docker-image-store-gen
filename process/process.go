package process

import (
	"os"
	"os/exec"
	"syscall"
)

func NewFirstLevelReExec() (*exec.Cmd, *os.File, error) {
	readPipe, writePipe, err := os.Pipe()
	if err != nil {
		return nil, nil, err
	}

	cmd := exec.Command(os.Args[0], os.Args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Unshareflags: syscall.CLONE_NEWNS | syscall.CLONE_NEWUSER,
	}
	cmd.ExtraFiles = []*os.File{readPipe}
	return cmd, writePipe, nil
}

func NewSecondLevelReExec() *exec.Cmd {
	cmd := exec.Command(os.Args[0], removeString(os.Args[1:], "-unshare")...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}

func removeString(s []string, unwanted string) []string {
	for i, v := range s {
		if v == unwanted {
			return append(s[:i], s[i+1:]...)
		}
	}
	return s
}

//go:build !windows

package winapi

import (
	"os/exec"
	"syscall"
)

func IsAdmin() bool                                { return false }
func Elevate([]string) error                       { return ErrSoWindows }
func SetDWord(string, string, uint32) error        { return ErrSoWindows }
func SetString(string, string, string) error       { return ErrSoWindows }
func SetBinary(string, string, []byte) error       { return ErrSoWindows }
func SetExpandString(string, string, string) error { return ErrSoWindows }
func GetString(string, string) (string, error)     { return "", ErrSoWindows }
func GetDWord(string, string) (uint64, error)      { return 0, ErrSoWindows }
func ValueNames(string) ([]string, error)          { return nil, ErrSoWindows }
func DeleteValue(string, string) error             { return ErrSoWindows }
func RefreshPath()                                 {}
func AddToMachinePath(string) error                { return ErrSoWindows }
func BroadcastSettingChange(string)                {}

func PrepareCmd(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM) }
}

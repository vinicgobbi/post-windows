//go:build windows

package winapi

import (
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// IsAdmin informa se o processo roda elevado (token de Administrador).
func IsAdmin() bool {
	return windows.GetCurrentProcessToken().IsElevated()
}

// Elevate reabre o executável atual com o verbo "runas" (prompt do UAC),
// numa janela de console nova. Devolve erro se o usuário recusar.
func Elevate(args []string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cwd, _ := os.Getwd()
	verb, _ := windows.UTF16PtrFromString("runas")
	file, _ := windows.UTF16PtrFromString(exe)
	params, _ := windows.UTF16PtrFromString(JoinArgs(args))
	dir, _ := windows.UTF16PtrFromString(cwd)
	return windows.ShellExecute(0, verb, file, params, dir, windows.SW_SHOWNORMAL)
}

func abrir(key string, acesso uint32) (registry.Key, error) {
	root, sub, err := SplitKey(key)
	if err != nil {
		return 0, err
	}
	base := registry.CURRENT_USER
	if root == "HKLM" {
		base = registry.LOCAL_MACHINE
	}
	if acesso&registry.SET_VALUE != 0 {
		k, _, err := registry.CreateKey(base, sub, acesso)
		return k, err
	}
	return registry.OpenKey(base, sub, acesso)
}

// SetDWord grava um valor DWORD, criando a chave se preciso.
func SetDWord(key, name string, v uint32) error {
	k, err := abrir(key, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	return k.SetDWordValue(name, v)
}

// SetString grava um valor REG_SZ, criando a chave se preciso.
func SetString(key, name, v string) error {
	k, err := abrir(key, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	return k.SetStringValue(name, v)
}

// SetBinary grava um valor REG_BINARY, criando a chave se preciso.
func SetBinary(key, name string, v []byte) error {
	k, err := abrir(key, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	return k.SetBinaryValue(name, v)
}

// SetExpandString grava um valor REG_EXPAND_SZ (ex.: o Path), criando a
// chave se preciso.
func SetExpandString(key, name, v string) error {
	k, err := abrir(key, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	return k.SetExpandStringValue(name, v)
}

// GetString lê um valor de texto (REG_SZ ou REG_EXPAND_SZ, sem expandir).
func GetString(key, name string) (string, error) {
	k, err := abrir(key, registry.QUERY_VALUE)
	if err != nil {
		return "", err
	}
	defer k.Close()
	s, _, err := k.GetStringValue(name)
	return s, err
}

// GetDWord lê um valor numérico.
func GetDWord(key, name string) (uint64, error) {
	k, err := abrir(key, registry.QUERY_VALUE)
	if err != nil {
		return 0, err
	}
	defer k.Close()
	v, _, err := k.GetIntegerValue(name)
	return v, err
}

// ValueNames lista os nomes dos valores de uma chave.
func ValueNames(key string) ([]string, error) {
	k, err := abrir(key, registry.QUERY_VALUE)
	if err != nil {
		return nil, err
	}
	defer k.Close()
	return k.ReadValueNames(-1)
}

// DeleteValue apaga um valor; não é erro se ele (ou a chave) não existir.
func DeleteValue(key, name string) error {
	root, sub, err := SplitKey(key)
	if err != nil {
		return err
	}
	base := registry.CURRENT_USER
	if root == "HKLM" {
		base = registry.LOCAL_MACHINE
	}
	// OpenKey, e não abrir(): apagar um valor não deve criar a chave.
	k, err := registry.OpenKey(base, sub, registry.SET_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer k.Close()
	if err := k.DeleteValue(name); err != nil && !errors.Is(err, registry.ErrNotExist) {
		return err
	}
	return nil
}

const (
	envMaquina = `HKLM\SYSTEM\CurrentControlSet\Control\Session Manager\Environment`
	envUsuario = `HKCU\Environment`
)

// RefreshPath relê o Path de máquina e de usuário do registro para o PATH
// deste processo — necessário para achar, já nesta execução, comandos
// recém-instalados pelo winget (fnm, php, rustup...).
func RefreshPath() {
	var partes []string
	for _, key := range []string{envMaquina, envUsuario} {
		if v, err := GetString(key, "Path"); err == nil && v != "" {
			if exp, err := registry.ExpandString(v); err == nil {
				v = exp
			}
			partes = append(partes, v)
		}
	}
	if len(partes) > 0 {
		os.Setenv("PATH", strings.Join(partes, ";"))
	}
}

// AddToMachinePath acrescenta dir ao Path de máquina, se ainda não estiver lá.
func AddToMachinePath(dir string) error {
	atual, err := GetString(envMaquina, "Path")
	if err != nil {
		return err
	}
	for _, p := range strings.Split(atual, ";") {
		if strings.EqualFold(strings.TrimRight(p, `\`), strings.TrimRight(dir, `\`)) {
			return nil
		}
	}
	return SetExpandString(envMaquina, "Path", strings.TrimRight(atual, ";")+";"+dir)
}

var procSendMessageTimeout = windows.NewLazySystemDLL("user32.dll").NewProc("SendMessageTimeoutW")

// BroadcastSettingChange avisa as janelas de que uma configuração mudou
// (WM_SETTINGCHANGE), do mesmo jeito que o app Configurações faz — ex.:
// "ImmersiveColorSet" para a troca de tema claro/escuro.
func BroadcastSettingChange(area string) {
	const (
		hwndBroadcast   = 0xffff
		wmSettingChange = 0x001A
		smtoAbortIfHung = 0x0002
	)
	p, err := windows.UTF16PtrFromString(area)
	if err != nil {
		return
	}
	var res uintptr
	procSendMessageTimeout.Call(hwndBroadcast, wmSettingChange, 0, uintptr(unsafe.Pointer(p)),
		smtoAbortIfHung, 2000, uintptr(unsafe.Pointer(&res)))
}

// PrepareCmd faz o comando rodar sem console visível (CREATE_NO_WINDOW): um
// console próprio e oculto, para a saída de programas que escrevem direto no
// console não bagunçar a TUI. Janelas gráficas (instaladores) continuam
// aparecendo normalmente. No cancelamento, encerra a árvore inteira.
func PrepareCmd(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NO_WINDOW}
	cmd.Cancel = func() error {
		return exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid)).Run()
	}
}

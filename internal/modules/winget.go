package modules

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/vinicgobbi/post-windows/internal/config"
	"github.com/vinicgobbi/post-windows/internal/runner"
)

// Códigos de saída do winget (doc/windows/package-manager/winget/returnCodes.md).
const (
	wingetUpdateNotApplicable   = -1978335189 // 0x8A15002B: já instalado, sem atualização
	wingetPackageAlreadyInstall = -1978335135 // 0x8A150061
	wingetInstallAlreadyInstall = -1978334963 // 0x8A15010D: outra versão já instalada
	wingetRebootToFinish        = -1978334967 // 0x8A150109: reinicie para concluir
	wingetRebootInitiated       = -1978334965 // 0x8A15010B
)

// wingetCache é a lista de pacotes instalados, montada uma vez só (cada
// "winget list" custa alguns segundos de sincronização de fonte).
type wingetCache struct {
	ids map[string]bool
	ok  bool // a lista foi lida (senão, cada checagem pergunta ao winget)
}

func (c *Ctx) instalados() *wingetCache {
	if c.winget != nil {
		return c.winget
	}
	c.winget = &wingetCache{ids: map[string]bool{}}
	// "winget export" é a única saída do winget legível por máquina (a
	// tabela do "winget list" trunca IDs longos com "…").
	arq := c.Temp("winget-export.json")
	c.Output("winget", "export", "-o", arq, "--accept-source-agreements")
	if data, err := os.ReadFile(arq); err == nil {
		if ids, err := parseWingetExport(data); err == nil {
			for _, id := range ids {
				c.winget.ids[strings.ToLower(id)] = true
			}
			c.winget.ok = true
		}
		os.Remove(arq)
	}
	return c.winget
}

func parseWingetExport(data []byte) ([]string, error) {
	var exp struct {
		Sources []struct {
			Packages []struct{ PackageIdentifier string }
		}
	}
	// O winget grava o JSON com BOM.
	data = []byte(strings.TrimPrefix(string(data), "\ufeff"))
	if err := json.Unmarshal(data, &exp); err != nil {
		return nil, err
	}
	var ids []string
	for _, s := range exp.Sources {
		for _, p := range s.Packages {
			ids = append(ids, p.PackageIdentifier)
		}
	}
	return ids, nil
}

func (c *Ctx) wingetInstalado(id string) bool {
	cache := c.instalados()
	if cache.ids[strings.ToLower(id)] {
		return true
	}
	if cache.ok {
		return false
	}
	return c.Probe("winget", "list", "--id", id, "-e", "--accept-source-agreements")
}

// installApp instala um app pelo winget (ou, se falhar, pelo Chocolatey,
// quando o app tem Choco). Falha de instalação vira aviso, não erro: um app
// que não instala não deve impedir os outros do módulo. Só devolve erro em
// cancelamento.
func (c *Ctx) installApp(app config.App) error {
	if c.wingetInstalado(app.ID) {
		c.Log.Successf("%s já está instalado.", app.Nome)
		return nil
	}

	c.Log.Infof("Instalando %s...", app.Nome)
	args := []string{"install", "--id", app.ID, "-e", "--silent",
		"--accept-source-agreements", "--accept-package-agreements", "--disable-interactivity"}
	if app.Source != "" {
		args = append(args, "--source", app.Source)
	}
	args = append(args, app.Args...)

	var code int
	var err error
	if app.UserScope {
		code, err = c.Deelevated(wingetExe(c), args...)
	} else {
		code, err = c.ExitCode("winget", args...)
	}
	if err != nil && c.Ctx.Err() != nil {
		return c.Ctx.Err()
	}

	switch {
	case err != nil:
		c.Log.Warnf("Não consegui executar o winget para %s: %v", app.Nome, err)
	case code == 0:
		c.Log.Successf("%s instalado.", app.Nome)
	case code == wingetUpdateNotApplicable || code == wingetPackageAlreadyInstall || code == wingetInstallAlreadyInstall:
		c.Log.Successf("%s já está instalado.", app.Nome)
	case code == wingetRebootToFinish || code == wingetRebootInitiated:
		c.Log.Warnf("%s instalado: reinicie o Windows para concluir a instalação.", app.Nome)
	default:
		c.Log.Warnf("winget não conseguiu instalar %s (código 0x%08X).", app.Nome, uint32(int32(code)))
		if app.Choco != "" {
			return c.chocoInstall(app.Choco, app.Nome)
		}
		return nil
	}
	if err == nil {
		c.winget.ids[strings.ToLower(app.ID)] = true
		c.RefreshPath()
	}
	return nil
}

func (c *Ctx) installApps(apps []config.App) error {
	for _, a := range apps {
		if err := c.installApp(a); err != nil {
			return err
		}
	}
	return nil
}

// wingetExe resolve o caminho real do winget.exe. "winget" não é um .exe de
// verdade no PATH: é um App Execution Alias (em %LOCALAPPDATA%\Microsoft\
// WindowsApps) que só é resolvido quando o processo nasce pelo shell — uma
// tarefa agendada (usada por Deelevated) recebe ERROR_FILE_NOT_FOUND
// (0x80070002) ao chamar "winget" pelo nome.
func wingetExe(c *Ctx) string {
	if p, err := c.PSOutput(`(Get-AppxPackage -Name Microsoft.DesktopAppInstaller | Select-Object -First 1).InstallLocation`); err == nil && p != "" {
		exe := p + `\winget.exe`
		if _, err := os.Stat(exe); err == nil {
			return exe
		}
	}
	return "winget"
}

// ensureChocolatey instala o Chocolatey só na primeira vez que algum app
// precisar dele.
func (c *Ctx) ensureChocolatey() error {
	if c.choco || runner.Exists("choco") {
		c.choco = true
		return nil
	}
	c.Log.Infof("Instalando o Chocolatey (fallback para apps fora do winget)...")
	err := c.PowerShell(`[System.Net.ServicePointManager]::SecurityProtocol = [System.Net.ServicePointManager]::SecurityProtocol -bor 3072
Invoke-Expression ((New-Object System.Net.WebClient).DownloadString('https://community.chocolatey.org/install.ps1'))`)
	if err != nil {
		return err
	}
	c.RefreshPath()
	c.choco = true
	c.Log.Successf("Chocolatey instalado.")
	return nil
}

func (c *Ctx) chocoInstall(pkg, nome string) error {
	if err := c.ensureChocolatey(); err != nil {
		if c.Ctx.Err() != nil {
			return err
		}
		c.Log.Warnf("Não consegui instalar o Chocolatey para %s: %v", nome, err)
		return nil
	}
	c.Log.Infof("Instalando %s via Chocolatey...", nome)
	if err := c.Run("choco", "install", pkg, "-y", "--no-progress"); err != nil {
		if c.Ctx.Err() != nil {
			return err
		}
		c.Log.Warnf("Falha ao instalar %s via Chocolatey: %v", nome, err)
		return nil
	}
	c.RefreshPath()
	c.Log.Successf("%s instalado (Chocolatey).", nome)
	return nil
}

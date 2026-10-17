package modules

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/vinicgobbi/post-windows/internal/config"
	"github.com/vinicgobbi/post-windows/internal/runner"
)

// moduloApps monta um módulo que só instala uma lista de apps do winget.
func moduloApps(id, title, desc string, apps []config.App) Module {
	return Module{ID: id, Title: title, Desc: desc, Run: func(c *Ctx) error {
		if err := c.installApps(apps); err != nil {
			return err
		}
		c.Log.Successf("%s: concluído.", title)
		return nil
	}}
}

var bootstrap = Module{
	ID:    "bootstrap",
	Title: "Preparar ambiente",
	Desc:  "Atualiza as fontes do winget e prepara NuGet/PSGallery para os demais módulos",
	Run: func(c *Ctx) error {
		c.Log.Infof("Preparando o ambiente (winget, NuGet, PSGallery)...")
		if err := c.Run("winget", "source", "update"); err != nil {
			return err
		}
		// O Install-Module do PSWindowsUpdate depende do provider NuGet; e
		// confiar na PSGallery evita o prompt "repositório não confiável".
		err := c.PowerShell(`if (-not (Get-PackageProvider -ListAvailable -Name NuGet -ErrorAction SilentlyContinue)) {
    Install-PackageProvider -Name NuGet -MinimumVersion 2.8.5.201 -Force | Out-Null
}
if ((Get-PSRepository -Name PSGallery).InstallationPolicy -ne 'Trusted') {
    Set-PSRepository -Name PSGallery -InstallationPolicy Trusted
}`)
		if err != nil {
			return err
		}
		c.Log.Successf("Ambiente preparado.")
		return nil
	},
}

var windowsUpdate = Module{
	ID:    "windows_update",
	Title: "Atualizar o Windows",
	Desc:  "Instala o módulo PSWindowsUpdate e aplica as atualizações pendentes do Windows Update",
	Deps:  []string{"bootstrap"},
	Run: func(c *Ctx) error {
		c.Log.Infof("Buscando atualizações do Windows (pode demorar)...")
		// -IgnoreReboot de propósito: reiniciar sozinho no meio da lista
		// derrubaria os módulos seguintes. O aviso de reinício fica no final.
		instalar := `Install-Module -Name PSWindowsUpdate -Force -Scope AllUsers`
		aplicar := `Install-WindowsUpdate -AcceptAll -IgnoreReboot -Confirm:$false | Out-Null
if (Get-WURebootStatus -Silent) { 'REBOOT' } else { 'OK' }`
		if c.DryRun {
			// Só lista o que seria instalado.
			instalar, aplicar = `'SEMMODULO'; return`, `'DRYRUN'`
		}
		out, err := c.PSOutput(`if (-not (Get-Module -ListAvailable -Name PSWindowsUpdate)) { ` + instalar + ` }
Import-Module PSWindowsUpdate
$updates = Get-WindowsUpdate -AcceptAll -ErrorAction SilentlyContinue
if (-not $updates) { 'NADA'; return }
$updates | ForEach-Object { 'UPD ' + $_.Title }
` + aplicar)
		if err != nil {
			return err
		}
		if strings.Contains(out, "SEMMODULO") {
			c.Log.Infof("[dry-run] Install-Module PSWindowsUpdate (sem o módulo, não dá para listar as atualizações)")
			return nil
		}
		for _, l := range strings.Split(out, "\n") {
			if t, ok := strings.CutPrefix(strings.TrimSpace(l), "UPD "); ok {
				c.Log.Infof("  %s", t)
			}
		}
		switch {
		case strings.Contains(out, "NADA"):
			c.Log.Successf("Nenhuma atualização pendente.")
		case strings.Contains(out, "DRYRUN"):
			c.Log.Infof("[dry-run] Install-WindowsUpdate -AcceptAll -IgnoreReboot")
		case strings.Contains(out, "REBOOT"):
			c.Log.Warnf("Atualizações instaladas: é necessário reiniciar o Windows para concluir (reinicie ao final).")
		default:
			c.Log.Successf("Atualizações do Windows instaladas.")
		}
		return nil
	},
}

var navegadorGit = Module{
	ID:    "navegador_git",
	Title: "Chrome + Git + Git Credential Manager",
	Desc:  "Instala o Google Chrome, o Git e o Git Credential Manager, com o Git ajustado para repositórios grandes (FSMonitor, cache de não rastreados) e caminhos longos",
	Run: func(c *Ctx) error {
		if err := c.installApps(config.NavegadorGit); err != nil {
			return err
		}
		// Caminhos longos: o limite antigo de 260 caracteres quebra
		// npm install/git clone com pastas muito aninhadas. Vale para os
		// programas que declaram suporte (o Git, com core.longpaths).
		if err := c.SetDWord(`HKLM\SYSTEM\CurrentControlSet\Control\FileSystem`, "LongPathsEnabled", 1); err != nil {
			return err
		}
		if !runner.Exists("git") {
			c.Log.Warnf("Git não encontrado no PATH; pulando os ajustes do Git.")
			return nil
		}
		// No Windows, o "git status" precisa varrer o repositório inteiro a
		// cada vez. O FSMonitor (embutido no Git for Windows) observa as
		// mudanças e responde direto; o untrackedCache evita revarrer as
		// pastas que não mudaram.
		for _, kv := range [][2]string{
			{"core.fsmonitor", "true"},
			{"core.untrackedCache", "true"},
			{"core.longpaths", "true"},
		} {
			if err := c.Run("git", "config", "--global", kv[0], kv[1]); err != nil {
				if c.Ctx.Err() != nil {
					return err
				}
				c.Log.Warnf("Não consegui definir %s no Git: %v", kv[0], err)
			}
		}
		c.Log.Successf("Chrome, Git e GCM instalados; Git com FSMonitor, cache de não rastreados e caminhos longos.")
		return nil
	},
}

var bitwarden = moduloApps("bitwarden", "Instalar Bitwarden",
	"Cliente desktop nativo do Bitwarden", []config.App{{ID: "Bitwarden.Bitwarden", Nome: "Bitwarden"}})

var comunicacaoMidia = moduloApps("comunicacao_midia", "Comunicação e mídia",
	"Telegram, Vesktop (Discord com Vencord já embutido), Spotify, WhatsApp, VLC e OBS Studio", config.ComunicacaoMidia)

var produtividade = moduloApps("produtividade", "Produtividade",
	"Obsidian, Ente Auth (2FA) e Gaphor", config.Produtividade)

var jogos = moduloApps("jogos", "Instalar launchers de jogos",
	"Steam, Heroic Games Launcher (Epic Games, GOG e Amazon Games num cliente só) e Prism Launcher (Minecraft)", config.Jogos)

var utilitariosRede = moduloApps("utilitarios_rede", "Utilitários de rede",
	"WinBox, qBittorrent e FileZilla Client (via Chocolatey, sem pacote no winget)", config.UtilitariosRede)

var logitechOptions = moduloApps("logitech_options", "Instalar Logi Options+",
	"Software oficial da Logitech para mouse/teclado (equivalente ao Solaar no Linux)",
	[]config.App{{ID: "Logitech.OptionsPlus", Nome: "Logi Options+"}})

var claudeCode = moduloApps("claude_code", "Instalar Claude Code",
	"CLI oficial da Anthropic para desenvolvimento assistido por IA no terminal",
	[]config.App{{ID: "Anthropic.ClaudeCode", Nome: "Claude Code"}})

var rustTools = moduloApps("rust_tools", "Instalar Rust, eza e topgrade",
	"rustup e os binários nativos de eza e topgrade (sem precisar compilar)", config.RustTools)

var devTools = Module{
	ID:    "dev_tools",
	Title: "Instalar ferramentas de desenvolvimento",
	Desc:  "VSCode, DBeaver, Postman, DevToys e fnm/Node.js LTS",
	Run: func(c *Ctx) error {
		if err := c.installApps(config.DevTools); err != nil {
			return err
		}

		c.Log.Infof("Instalando o Node.js LTS com o fnm...")
		if err := c.Run("fnm", "install", "--lts"); err != nil {
			if c.Ctx.Err() != nil {
				return err
			}
			c.Log.Warnf("fnm não conseguiu instalar o Node.js LTS: %v", err)
		} else if atual, err := c.Output("fnm", "current"); err == nil {
			if v := strings.TrimSpace(atual); v != "" && v != "none" {
				c.Try("fnm", "default", v)
			}
		}

		// Precisa estar no perfil (não só nesta sessão) para "node"/"npm"
		// funcionarem em terminais novos, como o "eval fnm env" do .zshrc.
		if err := c.addToProfiles("fnm env --use-on-cd | Out-String | Invoke-Expression"); err != nil {
			return err
		}
		c.Log.Successf("Ferramentas de desenvolvimento instaladas.")
		return nil
	},
}

var virtualbox = Module{
	ID:    "virtualbox",
	Title: "Instalar VirtualBox",
	Desc:  "Gerenciador de máquinas virtuais VirtualBox (equivalente ao GNOME Boxes/virt-manager); funciona em qualquer edição, inclusive Home",
	Run: func(c *Ctx) error {
		if out, err := c.PSOutput(`(Get-CimInstance Win32_Processor | Select-Object -First 1).VirtualizationFirmwareEnabled`); err == nil && strings.EqualFold(out, "False") {
			c.Log.Warnf("Virtualização por hardware (VT-x/AMD-V) não está habilitada na BIOS/UEFI — o VirtualBox pode não funcionar direito.")
		}
		if out, err := c.PSOutput(`(Get-WindowsOptionalFeature -Online -FeatureName Microsoft-Hyper-V-All -ErrorAction SilentlyContinue).State`); err == nil && out == "Enabled" {
			c.Log.Warnf("O Hyper-V está habilitado: o VirtualBox roda em cima dele (via API do Windows), o que funciona mas pode reduzir a performance.")
		}
		return c.installApp(config.App{ID: "Oracle.VirtualBox", Nome: "VirtualBox"})
	},
}

var tailscale = Module{
	ID:    "tailscale",
	Title: "Instalar Tailscale",
	Desc:  "Tailscale (VPN mesh) com o cliente gráfico nativo do Windows",
	Run: func(c *Ctx) error {
		// O cliente Windows já vem com ícone de bandeja e app gráfico —
		// diferente do Linux, não precisa de um "Trayscale" separado.
		if err := c.installApp(config.App{ID: "Tailscale.Tailscale", Nome: "Tailscale"}); err != nil {
			return err
		}
		c.Log.Infof("Abra o Tailscale pelo ícone da bandeja e faça login para conectar este PC à sua tailnet (o login é interativo).")
		return nil
	},
}

var ovpn = Module{
	ID:    "ovpn",
	Title: "Importar perfis OpenVPN",
	Desc:  "Instala o OpenVPN GUI e copia os .ovpn de .\\OVPN para a pasta de perfis monitorada por ele",
	Run: func(c *Ctx) error {
		if err := c.installApp(config.App{ID: "OpenVPNTechnologies.OpenVPN", Nome: "OpenVPN GUI"}); err != nil {
			return err
		}
		arquivos, _ := filepath.Glob(filepath.Join(c.OVPNDir, "*.ovpn"))
		if len(arquivos) == 0 {
			c.Log.Warnf("Nenhum arquivo .ovpn encontrado em %s — pulando importação.", c.OVPNDir)
			return nil
		}
		// O OpenVPN GUI lista cada .ovpn desta pasta como um perfil; DNS e
		// domínio do dhcp-option já são aplicados pelo próprio cliente.
		destino := filepath.Join(os.Getenv("ProgramFiles"), "OpenVPN", "config")
		for _, a := range arquivos {
			if err := c.CopyFile(a, filepath.Join(destino, filepath.Base(a))); err != nil {
				return err
			}
			c.Log.Successf("Perfil '%s' importado.", strings.TrimSuffix(filepath.Base(a), ".ovpn"))
		}
		c.Log.Infof("Abra o OpenVPN GUI (ícone na bandeja) para conectar — a autenticação é manual.")
		return nil
	},
}

// Package config guarda o catálogo de apps e as configurações compartilhadas
// entre módulos.
package config

// App é um pacote do winget (ID confirmado com "winget search").
type App struct {
	// ID vazio instala direto pelo Chocolatey (Choco), sem tentar o winget.
	ID   string
	Nome string
	// Source força a fonte do winget ("msstore" para apps da Microsoft Store).
	Source string
	// Args são argumentos extras para o "winget install" (ex.: --override).
	Args []string
	// UserScope instala sem elevação (ver runner.Deelevated): alguns
	// instaladores falham quando o winget roda como Administrador.
	UserScope bool
	// Choco é o pacote do Chocolatey usado quando o winget não consegue
	// instalar (app sem pacote no winget, ou manifest quebrado).
	Choco string
}

// Apps é a lista padrão de aplicativos (módulo "apps"), o equivalente à
// lista de Flatpaks do post-install.
var Apps = []App{
	{ID: "Postman.Postman", Nome: "Postman"},
	{ID: "DBeaver.DBeaver.Community", Nome: "DBeaver CE"},
	// O pacote "DevToys-app.DevToys" do winget é a build preview (2.0-preview.x)
	// apesar do nome não deixar isso claro — "9PGCV4V3BK4W" é a estável de
	// verdade (1.0.x), vem da Microsoft Store. No Linux, o equivalente é o
	// DevToolbox.
	{ID: "9PGCV4V3BK4W", Nome: "DevToys", Source: "msstore"},
	{ID: "Obsidian.Obsidian", Nome: "Obsidian"},
	{ID: "ente-io.auth-desktop", Nome: "Ente Auth"},
	{ID: "gaphor.gaphor", Nome: "Gaphor"},
	{ID: "TheDocumentFoundation.LibreOffice", Nome: "LibreOffice"},
	{ID: "ONLYOFFICE.DesktopEditors", Nome: "OnlyOffice Desktop Editors"},
	{ID: "Telegram.TelegramDesktop", Nome: "Telegram"},
	{ID: "Vencord.Vesktop", Nome: "Vesktop (Discord + Vencord já embutido)"},
	// Spotify e WhatsApp falham ao instalar via winget elevado: precisam
	// rodar como o usuário padrão. O WhatsApp é o equivalente do ZapZap.
	{ID: "Spotify.Spotify", Nome: "Spotify", UserScope: true},
	{ID: "WhatsApp.WhatsApp", Nome: "WhatsApp", UserScope: true},
	{ID: "VideoLAN.VLC", Nome: "VLC"},
	{ID: "OBSProject.OBSStudio", Nome: "OBS Studio"},
	{ID: "qBittorrent.qBittorrent", Nome: "qBittorrent"},
	// Sem pacote no winget: vai direto pelo Chocolatey.
	{Nome: "FileZilla Client", Choco: "filezilla"},
	{ID: "Mikrotik.Winbox", Nome: "WinBox"},
}

var Jogos = []App{
	{ID: "Valve.Steam", Nome: "Steam"},
	// O Heroic já cobre Epic Games, GOG e Amazon Games num cliente só — não
	// faz sentido instalar os launchers oficiais dessas lojas em paralelo.
	{ID: "HeroicGamesLauncher.HeroicGamesLauncher", Nome: "Heroic Games Launcher"},
	{ID: "PrismLauncher.PrismLauncher", Nome: "Prism Launcher"},
}

var Ferramentas = []App{
	{ID: "GitHub.cli", Nome: "GitHub CLI"},
	{ID: "Gitea.tea", Nome: "Gitea CLI (tea)"},
	{ID: "sharkdp.bat", Nome: "bat"},
	// Port do btop (sucessor do bashtop, do mesmo autor) para o Windows.
	{ID: "aristocratos.btop4win", Nome: "btop4win"},
	{ID: "Gyan.FFmpeg", Nome: "FFmpeg"},
	// O winget só tem o Nmap 7.80 (2019); o Chocolatey acompanha as releases.
	{Nome: "Nmap", Choco: "nmap"},
}

// RustTools são as ferramentas do módulo "rust" além do próprio Rust (que
// vem do mise). Diferente do Linux, eza e topgrade já têm binário nativo
// pré-compilado no winget: não precisa de toolchain C nem compilar via cargo.
var RustTools = []App{
	{ID: "eza-community.eza", Nome: "eza"},
	{ID: "topgrade-rs.topgrade", Nome: "topgrade"},
}

// Apps do post-install (Linux) sem equivalente/necessidade no Windows:
//   - Flatseal, Extension Manager, Ignition, Warehouse: gerenciam Flatpak e
//     o GNOME, que não existem no Windows.
//   - Mission Center: substituído pelo Gerenciador de Tarefas.
//   - Remmina: RDP já é nativo (mstsc).
//   - GNOME Boxes: substituído pelo VirtualBox (módulo "virtualbox").
//   - Gradia, Snapshot e Weather: a Ferramenta de Captura, a Câmera e o
//     Clima já vêm com o Windows.
//   - Trayscale: o cliente do Tailscale no Windows já tem ícone na bandeja.
//   - ProtonPlus: gerencia o Proton, sem sentido no Windows.
//   - Blip: sem pacote no winget nem no Chocolatey.
//   - Thunderbird: substituído pelo Outlook novo, que já vem com o Windows.
//   - Podman/Docker: não usados no Windows (exigiriam o WSL2).
//   - cargo-update: no Linux o topgrade o usa para atualizar o que veio do
//     "cargo install"; aqui nada é instalado pelo cargo (eza e topgrade vêm
//     do winget, que o topgrade atualiza).
//   - jq e sqlcmd (mssql-tools): não usados; do SQL Server basta o driver ODBC.

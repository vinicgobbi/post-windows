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

var NavegadorGit = []App{
	{ID: "Google.Chrome", Nome: "Google Chrome"},
	{ID: "Mozilla.Firefox", Nome: "Mozilla Firefox"},
	{ID: "Git.Git", Nome: "Git"},
	{ID: "Git.GCM", Nome: "Git Credential Manager"},
}

var DevTools = []App{
	// O instalador do VSCode (Inno Setup) só habilita "Adicionar ao PATH" por
	// padrão no silent install — as opções de menu de contexto do Explorer
	// ("Abrir com o Code" em arquivos e pastas) ficam desmarcadas a menos que
	// sejam forçadas via /MERGETASKS.
	{ID: "Microsoft.VisualStudioCode", Nome: "Visual Studio Code",
		Args: []string{"--override", "/VERYSILENT /MERGETASKS=addcontextmenufiles,addcontextmenufolders"}},
	{ID: "DBeaver.DBeaver.Community", Nome: "DBeaver CE"},
	{ID: "Postman.Postman", Nome: "Postman"},
	// O pacote "DevToys-app.DevToys" do winget é a build preview (2.0-preview.x)
	// apesar do nome não deixar isso claro — "9PGCV4V3BK4W" é a estável de
	// verdade (1.0.x), vem da Microsoft Store.
	{ID: "9PGCV4V3BK4W", Nome: "DevToys", Source: "msstore"},
	{ID: "Schniz.fnm", Nome: "fnm (Fast Node Manager)"},
}

var ComunicacaoMidia = []App{
	{ID: "Telegram.TelegramDesktop", Nome: "Telegram"},
	{ID: "Vencord.Vesktop", Nome: "Vesktop (Discord + Vencord já embutido)"},
	// Spotify e WhatsApp falham ao instalar via winget elevado: precisam
	// rodar como o usuário padrão.
	{ID: "Spotify.Spotify", Nome: "Spotify", UserScope: true},
	{ID: "WhatsApp.WhatsApp", Nome: "WhatsApp", UserScope: true},
	{ID: "VideoLAN.VLC", Nome: "VLC"},
	{ID: "OBSProject.OBSStudio", Nome: "OBS Studio"},
}

var Produtividade = []App{
	{ID: "Obsidian.Obsidian", Nome: "Obsidian"},
	{ID: "ente-io.auth-desktop", Nome: "Ente Auth"},
	{ID: "gaphor.gaphor", Nome: "Gaphor"},
}

var Jogos = []App{
	{ID: "Valve.Steam", Nome: "Steam"},
	// O Heroic já cobre Epic Games, GOG e Amazon Games num cliente só — não
	// faz sentido instalar os launchers oficiais dessas lojas em paralelo.
	{ID: "HeroicGamesLauncher.HeroicGamesLauncher", Nome: "Heroic Games Launcher"},
	{ID: "PrismLauncher.PrismLauncher", Nome: "Prism Launcher"},
}

var UtilitariosRede = []App{
	{ID: "Mikrotik.Winbox", Nome: "WinBox"},
	{ID: "qBittorrent.qBittorrent", Nome: "qBittorrent"},
	// Sem pacote no winget: vai direto pelo Chocolatey.
	{Nome: "FileZilla Client", Choco: "filezilla"},
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

var RustTools = []App{
	// Diferente do Linux, eza e topgrade já têm binário nativo pré-compilado
	// no winget — não precisa de toolchain C nem compilar via cargo.
	{ID: "Rustlang.Rustup", Nome: "rustup"},
	{ID: "eza-community.eza", Nome: "eza"},
	{ID: "topgrade-rs.topgrade", Nome: "topgrade"},
}

var Office = []App{
	{ID: "TheDocumentFoundation.LibreOffice", Nome: "LibreOffice"},
	{ID: "ONLYOFFICE.DesktopEditors", Nome: "OnlyOffice Desktop Editors"},
}

// DotfilesRepoURL é o repositório de onde vem a config do Oh My Posh (pasta
// oh-my-posh/: tema, aliases, editor).
const DotfilesRepoURL = "https://github.com/vinicgobbi/Dotfiles.git"

// Apps do inventário Linux sem equivalente/necessidade no Windows:
//   - Flatseal, Extension Manager, Ignition, Warehouse: gerenciam Flatpak,
//     que não existe no Windows.
//   - Mission Center: substituído pelo Gerenciador de Tarefas.
//   - Remmina: RDP já é nativo (mstsc).
//   - ProtonPlus: gerencia o Proton, sem sentido no Windows.
//   - Thunderbird: substituído pelo Outlook novo, que já vem com o Windows
//     (o debloat o mantém).
//   - Docker: não usado no Windows (exigiria o WSL2).
//   - jq e sqlcmd (mssql-tools): não usados; do SQL Server basta o driver ODBC.

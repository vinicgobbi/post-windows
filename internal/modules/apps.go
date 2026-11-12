package modules

import (
	"github.com/vinicgobbi/post-windows/internal/config"
)

// moduloApps monta um módulo que só instala uma lista de apps do winget.
func moduloApps(id, grupo, title, desc string, lista []config.App) Module {
	return Module{ID: id, Grupo: grupo, Title: title, Desc: desc, Run: func(c *Ctx) error {
		if err := c.installApps(lista); err != nil {
			return err
		}
		c.Log.Successf("%s: concluído.", title)
		return nil
	}}
}

var chrome = moduloApps("chrome", GrupoAplicativos, "Google Chrome",
	"Instala o Google Chrome (o instalador oficial, que se atualiza sozinho)",
	[]config.App{{ID: "Google.Chrome", Nome: "Google Chrome"}})

// O Thunderbird do post-install fica de fora: o Outlook novo já vem com o
// Windows (veja config.Apps).
var firefox = moduloApps("firefox", GrupoAplicativos, "Firefox",
	"Instala o Firefox (o instalador oficial da Mozilla, que se atualiza sozinho)",
	[]config.App{{ID: "Mozilla.Firefox", Nome: "Mozilla Firefox"}})

var bitwarden = moduloApps("bitwarden", GrupoAplicativos, "Bitwarden",
	"Instala o Bitwarden desktop (necessário para integração nativa com o navegador)",
	[]config.App{{ID: "Bitwarden.Bitwarden", Nome: "Bitwarden"}})

var apps = moduloApps("apps", GrupoAplicativos, "Aplicativos",
	"A lista de aplicativos (o equivalente aos Flatpaks do Linux): Postman, DBeaver, DevToys, Obsidian, Ente Auth, Gaphor, LibreOffice, OnlyOffice, Telegram, Vesktop, Spotify, WhatsApp, VLC, OBS Studio, qBittorrent, FileZilla e WinBox",
	config.Apps)

var jogos = moduloApps("jogos", GrupoAplicativos, "Jogos",
	"Steam, Heroic Games Launcher (Epic Games, GOG e Amazon Games num cliente só) e Prism Launcher (Minecraft)",
	config.Jogos)

var logitech = moduloApps("logitech", GrupoAplicativos, "Logi Options+ (Logitech)",
	"Software oficial da Logitech para mouse/teclado (o equivalente ao Solaar no Linux)",
	[]config.App{{ID: "Logitech.OptionsPlus", Nome: "Logi Options+"}})

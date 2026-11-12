package modules

import (
	"errors"
	"io/fs"
	"strings"

	"github.com/vinicgobbi/post-windows/internal/config"
	"github.com/vinicgobbi/post-windows/internal/winpath"
)

var ferramentas = Module{
	ID:    "ferramentas",
	Grupo: GrupoTerminal,
	Title: "Ferramentas de terminal",
	Desc:  "GitHub CLI, Gitea CLI (tea), bat, btop4win, FFmpeg, Nmap (Chocolatey) e Python com pipx",
	Run: func(c *Ctx) error {
		if err := c.installApps(config.Ferramentas); err != nil {
			return err
		}
		// PrependPath=1 no instalador do pacote: python/py já ficam no PATH.
		if err := c.installApp(config.App{ID: "Python.Python.3.14", Nome: "Python 3.14"}); err != nil {
			return err
		}

		// Instrução oficial do pipx para Windows: via pip, no perfil do
		// usuário; o ensurepath põe a pasta dos apps do pipx no PATH dele.
		c.Log.Infof("Instalando o pipx...")
		err := step(
			func() error { return c.Run("py", "-m", "pip", "install", "--user", "--upgrade", "pipx") },
			func() error { return c.Run("py", "-m", "pipx", "ensurepath") },
		)
		if err := c.avisarFalha(err, "Não consegui instalar o pipx"); err != nil {
			return err
		}
		c.Log.Successf("Ferramentas de terminal instaladas.")
		return nil
	},
}

var shell = Module{
	ID:    "shell",
	Grupo: GrupoTerminal,
	Title: "PowerShell 7 e dotfiles",
	Desc:  "PowerShell 7 + Oh My Posh com a config dos dotfiles pessoais (tema, aliases, editor) em ~\\.config\\oh-my-posh e a fonte CaskaydiaCove Nerd Font",
	Run: func(c *Ctx) error {
		// Equivalente ao Zsh + Oh My Zsh do Linux, nativo do Windows (sem
		// WSL). O Git clona os dotfiles.
		if err := c.installApps([]config.App{
			{ID: "Microsoft.PowerShell", Nome: "PowerShell 7"},
			{ID: "JanDeDobbeleer.OhMyPosh", Nome: "Oh My Posh"},
			{ID: "Git.Git", Nome: "Git"},
		}); err != nil {
			return err
		}
		if c.Exists("oh-my-posh") {
			if err := c.avisarFalha(c.Run("oh-my-posh", "font", "install", "CascadiaCode"),
				"Não consegui instalar a fonte CascadiaCode Nerd Font"); err != nil {
				return err
			}
		}

		destino := winpath.Join(c.Getenv("USERPROFILE"), ".config", "oh-my-posh")
		pessoal, err := copiarConfigOhMyPosh(c, destino)
		if err != nil {
			return err
		}
		linha := "oh-my-posh init pwsh | Invoke-Expression"
		if pessoal {
			linha = `. "` + winpath.Join(destino, "profile.ps1") + `"`
		}
		if err := c.addToProfiles(linha); err != nil {
			return err
		}

		c.Log.Infof("Configure a fonte 'CaskaydiaCove Nerd Font' no terminal (Windows Terminal: Configurações > Perfil > Aparência > Fonte) para os ícones do tema aparecerem.")
		c.Log.Successf("PowerShell 7 instalado e perfis (5.1 e 7) configurados com Oh My Posh.")
		return nil
	},
}

// copiarConfigOhMyPosh copia só a pasta oh-my-posh/ do repositório de
// dotfiles (no commit fixado em pins.go) para destino: clona num diretório
// temporário, copia e descarta o clone. É uma cópia simples (sem git), de
// propósito: se o repositório de projetos for apagado, a config do prompt
// continua intacta; mudanças no repositório só chegam rodando este módulo
// de novo.
func copiarConfigOhMyPosh(c *Ctx, destino string) (bool, error) {
	if !c.Exists("git") && !c.DryRun {
		c.Log.Warnf("Git não encontrado; Oh My Posh fica no tema padrão.")
		return false, nil
	}
	clone := c.Temp("dotfiles")
	if err := clonarFixado(c, dotfilesRepo, dotfilesCommit, clone); err != nil {
		if c.Ctx.Err() != nil {
			return false, err
		}
		c.Log.Warnf("Não consegui clonar %s; Oh My Posh fica no tema padrão.", dotfilesRepo)
		return false, nil
	}
	defer c.RemoveAll(clone)
	if c.DryRun {
		c.Log.Infof("[dry-run] copiar %s -> %s", winpath.Join(clone, "oh-my-posh"), destino)
		return true, nil
	}

	origem := winpath.Join(clone, "oh-my-posh")
	if !c.FileExists(origem) {
		c.Log.Warnf("O repositório de dotfiles não tem a pasta 'oh-my-posh'; Oh My Posh fica no tema padrão.")
		return false, nil
	}
	err := c.WalkDir(origem, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, ok := winpath.Rel(origem, p)
		if !ok || strings.HasPrefix(rel, ".git") {
			return nil
		}
		return c.CopyFile(p, winpath.Join(destino, rel))
	})
	return err == nil, err
}

// perfis devolve o $PROFILE do Windows PowerShell 5.1 e do PowerShell 7. Os
// dois ficam em Documentos, que pode estar redirecionado para o OneDrive —
// por isso a pasta vem do próprio Windows, e não de %USERPROFILE%\Documents.
func (c *Ctx) perfis() ([]string, error) {
	docs, err := c.PSOutput(`[Environment]::GetFolderPath('MyDocuments')`)
	if err != nil {
		return nil, err
	}
	if docs == "" {
		return nil, errors.New("pasta Documentos não encontrada")
	}
	return []string{
		winpath.Join(docs, "WindowsPowerShell", "Microsoft.PowerShell_profile.ps1"),
		winpath.Join(docs, "PowerShell", "Microsoft.PowerShell_profile.ps1"),
	}, nil
}

// addToProfiles acrescenta uma linha aos perfis do PowerShell 5.1 e 7 (se
// ainda não estiver lá).
func (c *Ctx) addToProfiles(linha string) error {
	ps, err := c.perfis()
	if err != nil {
		return err
	}
	for _, p := range ps {
		if err := c.AppendLine(p, linha); err != nil {
			return err
		}
	}
	return nil
}

// removeFromProfiles tira dos perfis do PowerShell 5.1 e 7 uma linha que
// uma versão anterior do post-windows tinha escrito.
func (c *Ctx) removeFromProfiles(linha string) error {
	ps, err := c.perfis()
	if err != nil {
		return err
	}
	for _, p := range ps {
		if err := c.RemoveLine(p, linha); err != nil {
			return err
		}
	}
	return nil
}

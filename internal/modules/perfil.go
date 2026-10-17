package modules

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/vinicgobbi/post-windows/internal/config"
	"github.com/vinicgobbi/post-windows/internal/runner"
)

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
		filepath.Join(docs, "WindowsPowerShell", "Microsoft.PowerShell_profile.ps1"),
		filepath.Join(docs, "PowerShell", "Microsoft.PowerShell_profile.ps1"),
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

var powershellProfile = Module{
	ID:    "powershell_profile",
	Title: "Configurar perfil do PowerShell",
	Desc:  "PowerShell 7 + Oh My Posh com a config pessoal (tema, aliases, editor) copiada para ~\\.config\\oh-my-posh + fonte Nerd Font",
	Deps:  []string{"navegador_git"},
	Run: func(c *Ctx) error {
		// Equivalente ao Zsh + Oh My Zsh do Linux, nativo do Windows (sem WSL).
		if err := c.installApps([]config.App{
			{ID: "Microsoft.PowerShell", Nome: "PowerShell 7"},
			{ID: "JanDeDobbeleer.OhMyPosh", Nome: "Oh My Posh"},
		}); err != nil {
			return err
		}
		if runner.Exists("oh-my-posh") {
			if err := c.Run("oh-my-posh", "font", "install", "CascadiaCode"); err != nil {
				if c.Ctx.Err() != nil {
					return err
				}
				c.Log.Warnf("Não consegui instalar a fonte CascadiaCode Nerd Font: %v", err)
			}
		}

		destino := filepath.Join(os.Getenv("USERPROFILE"), ".config", "oh-my-posh")
		pessoal, err := copiarConfigOhMyPosh(c, destino)
		if err != nil {
			return err
		}
		linha := "oh-my-posh init pwsh | Invoke-Expression"
		if pessoal {
			linha = `. "` + filepath.Join(destino, "profile.ps1") + `"`
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
// dotfiles para destino: clona num diretório temporário, copia e descarta o
// clone. É uma cópia simples (sem git), de propósito: se o repositório de
// projetos for apagado, a config do prompt continua intacta; mudanças no
// repositório só chegam rodando este módulo de novo.
func copiarConfigOhMyPosh(c *Ctx, destino string) (bool, error) {
	if !runner.Exists("git") {
		c.Log.Warnf("Git não encontrado; Oh My Posh fica no tema padrão.")
		return false, nil
	}
	clone := c.Temp("dotfiles")
	if err := c.Run("git", "clone", "--depth", "1", "--quiet", config.DotfilesRepoURL, clone); err != nil {
		if c.Ctx.Err() != nil {
			return false, err
		}
		c.Log.Warnf("Não consegui clonar %s; Oh My Posh fica no tema padrão.", config.DotfilesRepoURL)
		return false, nil
	}
	defer c.RemoveAll(clone)
	if c.DryRun {
		c.Log.Infof("[dry-run] copiar %s\\oh-my-posh -> %s", clone, destino)
		return true, nil
	}

	origem := filepath.Join(clone, "oh-my-posh")
	if _, err := os.Stat(origem); err != nil {
		c.Log.Warnf("O repositório de dotfiles não tem a pasta 'oh-my-posh'; Oh My Posh fica no tema padrão.")
		return false, nil
	}
	err := filepath.WalkDir(origem, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(origem, p)
		if strings.HasPrefix(rel, ".git") {
			return nil
		}
		return c.CopyFile(p, filepath.Join(destino, rel))
	})
	return err == nil, err
}

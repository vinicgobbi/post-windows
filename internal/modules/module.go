// Package modules contém as etapas da pós-instalação. Cada arquivo define um
// grupo de módulos; a ordem de execução é a ordem da lista em All.
package modules

import (
	"fmt"

	"github.com/vinicgobbi/post-windows/internal/runner"
	"github.com/vinicgobbi/post-windows/internal/sysinfo"
)

type Module struct {
	ID    string
	Title string
	Desc  string
	// Deps são IDs de módulos que precisam rodar antes deste (ex.: o perfil
	// do PowerShell clona os dotfiles com o Git de "navegador_git").
	Deps []string
	Run  func(c *Ctx) error
}

// Ctx é o que cada módulo recebe para trabalhar.
type Ctx struct {
	*runner.Runner
	Sys     *sysinfo.Info
	OVPNDir string
	// DevDriveLetter e DevDriveGB são a letra e o tamanho máximo do Dev
	// Drive.
	DevDriveLetter string
	DevDriveGB     int

	winget   *wingetCache
	choco    bool   // Chocolatey já garantido nesta execução
	projetos string // pasta de projetos já resolvida (ver projetosDir)
}

// All devolve todos os módulos, na ordem de execução.
func All() []Module {
	return []Module{
		bootstrap,
		windowsUpdate,
		// Antes de quem instala npm/Composer/Rust, para os caches já
		// nascerem no Dev Drive.
		devDrive,
		navegadorGit,
		office,
		devTools,
		phpSqlsrv,
		bitwarden,
		comunicacaoMidia,
		produtividade,
		jogos,
		utilitariosRede,
		virtualbox,
		tailscale,
		logitechOptions,
		ovpn,
		claudeCode,
		ferramentas,
		ruby,
		deployDev,
		rustTools,
		powershellProfile,
		customizacaoWindows,
		performance,
		debloat,
		limpeza,
	}
}

// step encadeia passos, parando no primeiro erro.
func step(fns ...func() error) error {
	for _, fn := range fns {
		if err := fn(); err != nil {
			return err
		}
	}
	return nil
}

func errorf(format string, a ...any) error { return fmt.Errorf(format, a...) }

// Package modules contém as etapas da pós-instalação. Cada módulo pertence a
// um grupo e é autocontido (instala o que usa, então pode ser marcado ou
// desmarcado sozinho); a ordem de execução é a ordem da lista em All. Os IDs
// são os mesmos do post-install (Linux) quando o módulo faz a mesma coisa.
package modules

import (
	"fmt"
	"time"

	"github.com/vinicgobbi/post-windows/internal/runner"
	"github.com/vinicgobbi/post-windows/internal/sysinfo"
)

// Grupos, na ordem em que aparecem na interface e rodam.
const (
	GrupoSistema         = "Sistema"
	GrupoTerminal        = "Terminal"
	GrupoAplicativos     = "Aplicativos"
	GrupoDesenvolvimento = "Desenvolvimento"
	GrupoRede            = "Rede e virtualização"
	GrupoWindows         = "Windows e aparência"
	GrupoFinalizacao     = "Finalização"
)

type Module struct {
	ID    string
	Grupo string
	Title string
	Desc  string
	// Deps são IDs de módulos que precisam rodar antes deste (ex.: o Node
	// vem do mise).
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
	// Wallpaper é o caminho da imagem escolhida (vazio = não alterar).
	Wallpaper string
	// Agora devolve a hora atual (nil = time.Now); os testes golden fixam.
	Agora func() time.Time

	winget   *wingetCache
	choco    bool   // Chocolatey já garantido nesta execução
	projetos string // pasta de projetos já resolvida (ver projetosDir)
}

// Escolhas é o que o usuário decide antes da execução (na TUI ou no modo
// texto).
type Escolhas struct {
	Wallpaper string
}

func (c *Ctx) agora() time.Time {
	if c.Agora != nil {
		return c.Agora()
	}
	return time.Now()
}

// All devolve todos os módulos, na ordem de execução (agrupados por Grupo).
func All() []Module {
	return []Module{
		// Sistema. O Dev Drive vem antes de quem instala npm/Composer/Rust,
		// para os caches já nascerem nele.
		bootstrap,
		atualizacao,
		devDrive,
		// Terminal
		ferramentas,
		shell,
		// Aplicativos
		chrome,
		firefox,
		bitwarden,
		apps,
		jogos,
		logitech,
		// Desenvolvimento
		gitGCM,
		vscode,
		golang,
		mise,
		node,
		phpSQLServer,
		rust,
		ruby,
		claudeCode,
		deployDev,
		// Rede e virtualização
		tailscale,
		ovpn,
		virtualbox,
		// Windows e aparência
		papelParede,
		customizacaoWindows,
		performance,
		// Finalização
		limpeza,
	}
}

// Renomeados são os IDs de versões anteriores do post-windows (antes de
// seguirem os do post-install), com os módulos que fazem o mesmo hoje.
// --modules e --skip aceitam os antigos, com um aviso.
var Renomeados = map[string][]string{
	"windows_update":     {"atualizacao"},
	"navegador_git":      {"chrome", "firefox", "git_gcm"},
	"office":             {"apps"},
	"dev_tools":          {"vscode", "apps", "node"},
	"php_sqlsrv":         {"php_sqlserver"},
	"comunicacao_midia":  {"apps"},
	"produtividade":      {"apps"},
	"utilitarios_rede":   {"apps"},
	"logitech_options":   {"logitech"},
	"rust_tools":         {"rust"},
	"powershell_profile": {"shell"},
}

// Removidos são IDs de módulos que deixaram de existir. --modules e --skip
// os ignoram, com um aviso, para não quebrar quem ainda os passa.
var Removidos = map[string]bool{
	"debloat": true,
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

// avisarFalha transforma o erro de um passo opcional em aviso; só um
// cancelamento continua sendo erro.
func (c *Ctx) avisarFalha(err error, format string, a ...any) error {
	if err == nil {
		return nil
	}
	if c.Ctx.Err() != nil {
		return err
	}
	c.Log.Warnf(format+": %v", append(a, err)...)
	return nil
}

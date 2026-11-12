package modules

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/vinicgobbi/post-windows/internal/config"
	"github.com/vinicgobbi/post-windows/internal/winpath"
)

var gitGCM = Module{
	ID:    "git_gcm",
	Grupo: GrupoDesenvolvimento,
	Title: "Git e Git Credential Manager",
	Desc:  "Git e o Git Credential Manager (credenciais no Gerenciador de Credenciais do Windows), com o Git ajustado para repositórios grandes (FSMonitor, cache de não rastreados) e caminhos longos (também liberados no Windows)",
	Run: func(c *Ctx) error {
		if err := c.installApps([]config.App{
			{ID: "Git.Git", Nome: "Git"},
			{ID: "Git.GCM", Nome: "Git Credential Manager"},
		}); err != nil {
			return err
		}
		// Caminhos longos: o limite antigo de 260 caracteres quebra
		// npm install/git clone com pastas muito aninhadas. Vale para os
		// programas que declaram suporte (o Git, com core.longpaths).
		if err := c.SetDWord(`HKLM\SYSTEM\CurrentControlSet\Control\FileSystem`, "LongPathsEnabled", 1); err != nil {
			return err
		}
		if !c.Exists("git") && !c.DryRun {
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
			if err := c.avisarFalha(c.Run("git", "config", "--global", kv[0], kv[1]),
				"Não consegui definir %s no Git", kv[0]); err != nil {
				return err
			}
		}
		c.Log.Successf("Git e GCM instalados; Git com FSMonitor, cache de não rastreados e caminhos longos.")
		return nil
	},
}

var vscode = moduloApps("vscode", GrupoDesenvolvimento, "Visual Studio Code",
	"VSCode pelo instalador oficial da Microsoft, com \"Abrir com o Code\" no menu de contexto do Explorer",
	[]config.App{
		// O instalador do VSCode (Inno Setup) só habilita "Adicionar ao
		// PATH" por padrão no silent install — as opções de menu de
		// contexto do Explorer ("Abrir com o Code" em arquivos e pastas)
		// ficam desmarcadas a menos que sejam forçadas via /MERGETASKS. É o
		// equivalente do "Abrir com o VSCode" do Nautilus.
		{ID: "Microsoft.VisualStudioCode", Nome: "Visual Studio Code",
			Args: []string{"--override", "/VERYSILENT /MERGETASKS=addcontextmenufiles,addcontextmenufolders"}},
	})

var golang = moduloApps("go", GrupoDesenvolvimento, "Go",
	"Go pelo instalador oficial (go.dev)",
	[]config.App{{ID: "GoLang.Go", Nome: "Go"}})

// shimsMise é onde o mise põe os atalhos (shims) das ferramentas. No
// Windows, a documentação do mise recomenda os shims no PATH em vez do
// "mise activate": valem em qualquer terminal (PowerShell 5.1 e 7, cmd) e
// também para os programas abertos fora dele (VSCode, tarefas).
func shimsMise(c *Ctx) string {
	return winpath.Join(c.Getenv("LOCALAPPDATA"), "mise", "shims")
}

var mise = Module{
	ID:    "mise",
	Grupo: GrupoDesenvolvimento,
	Title: "mise",
	Desc:  "Gerenciador de versões de ferramentas (Node, pnpm, Rust...) pelo winget, com os shims no PATH do usuário",
	Run: func(c *Ctx) error {
		if err := c.installApp(config.App{ID: "jdx.mise", Nome: "mise"}); err != nil {
			return err
		}
		if err := c.AddToUserPath(shimsMise(c)); err != nil {
			return err
		}
		c.Log.Successf("mise instalado, com os shims em %s no PATH.", shimsMise(c))
		return nil
	},
}

// usarMise grava as ferramentas como globais do usuário
// (~\.config\mise\config.toml) e instala, recriando os shims.
func usarMise(c *Ctx, ferramentas ...string) error {
	return step(
		func() error { return c.Run("mise", append([]string{"use", "-g"}, ferramentas...)...) },
		func() error { return c.Run("mise", "reshim") },
	)
}

// linhaFnm é a ativação do fnm que versões anteriores punham nos perfis do
// PowerShell (o Node agora vem do mise).
const linhaFnm = "fnm env --use-on-cd | Out-String | Invoke-Expression"

var node = Module{
	ID:    "node",
	Grupo: GrupoDesenvolvimento,
	Title: "Node.js e pnpm (mise)",
	Desc:  "Node LTS e pnpm pelo mise, para o usuário",
	Deps:  []string{"mise"},
	Run: func(c *Ctx) error {
		c.Log.Infof("Instalando o Node.js LTS e o pnpm pelo mise...")
		if err := usarMise(c, "node@lts", "pnpm@latest"); err != nil {
			return err
		}
		// Execuções anteriores usavam o fnm: sem a ativação dele nos perfis,
		// o "node" de um terminal novo é o do mise.
		if err := c.removeFromProfiles(linhaFnm); err != nil {
			return err
		}
		if c.wingetInstalado("Schniz.fnm") {
			c.Log.Infof("O fnm (de uma versão anterior) não é mais usado; para removê-lo: winget uninstall Schniz.fnm")
		}
		c.Log.Successf("Node.js LTS e pnpm instalados pelo mise.")
		return nil
	},
}

var rust = Module{
	ID:    "rust",
	Grupo: GrupoDesenvolvimento,
	Title: "Rust, eza e topgrade",
	Desc:  "Rust stable pelo mise e os binários nativos de eza e topgrade (sem precisar compilar)",
	Deps:  []string{"mise"},
	Run: func(c *Ctx) error {
		c.Log.Infof("Instalando o Rust stable pelo mise...")
		if err := usarMise(c, "rust@stable"); err != nil {
			return err
		}
		if err := c.installApps(config.RustTools); err != nil {
			return err
		}
		// O toolchain padrão no Windows (MSVC) usa o linker do Visual
		// Studio para compilar; o vswhere vem com qualquer instalação dele.
		vswhere := winpath.Join(c.Getenv("ProgramFiles(x86)"), "Microsoft Visual Studio", "Installer", "vswhere.exe")
		if !c.FileExists(vswhere) {
			c.Log.Infof("Para compilar projetos Rust, instale o Visual Studio Build Tools com \"Desenvolvimento para desktop com C++\" (o linker do toolchain MSVC).")
		}
		c.Log.Successf("Rust, eza e topgrade instalados.")
		return nil
	},
}

var ruby = Module{
	ID:    "ruby",
	Grupo: GrupoDesenvolvimento,
	Title: "Ruby, Bundler e Jekyll",
	Desc:  "RubyInstaller com DevKit (MSYS2 + toolchain para gems nativas), Bundler e Jekyll; o Bundler instala as gems de cada projeto em vendor/bundle",
	Run: func(c *Ctx) error {
		// O instalador roda o "ridk install" ao final por padrão, num console
		// que espera Enter e trava o silent install; ele roda logo abaixo,
		// com a opção já escolhida. O Ruby fica no RubyInstaller (e não no
		// mise, como no Linux): é o caminho estável do Ruby no Windows.
		app := config.App{ID: "RubyInstallerTeam.RubyWithDevKit.3.4", Nome: "Ruby 3.4 (com DevKit)",
			Args: []string{"--override", "/VERYSILENT /NORESTART /ALLUSERS /MERGETASKS=!ridkinstall"}}
		if err := c.installApp(app); err != nil {
			return err
		}

		// Opção 3: MSYS2 atualizado + toolchain MinGW (o http_parser.rb,
		// dependência do "jekyll serve", compila na instalação).
		c.Log.Infof("Instalando a toolchain MSYS2/MinGW do Ruby (ridk install 3)...")
		if err := c.avisarFalha(c.Run("ridk", "install", "3"),
			"ridk install falhou; gems com extensão nativa podem não compilar"); err != nil {
			return err
		}

		// "path vendor/bundle" (global): o bundle install de cada projeto grava
		// as gems dentro do próprio projeto, como no Linux.
		c.Log.Infof("Instalando Bundler e Jekyll...")
		err := step(
			func() error { return c.Run("gem", "install", "--no-document", "bundler", "jekyll") },
			func() error { return c.Run("bundle", "config", "set", "--global", "path", "vendor/bundle") },
		)
		if err != nil {
			return err
		}
		c.Log.Successf("Ruby, Bundler e Jekyll instalados.")
		return nil
	},
}

var claudeCode = moduloApps("claude_code", GrupoDesenvolvimento, "Claude Code",
	"CLI oficial da Anthropic para desenvolvimento assistido por IA no terminal",
	[]config.App{{ID: "Anthropic.ClaudeCode", Nome: "Claude Code"}})

// O deploy-dev se instala sozinho (install copia para
// %LOCALAPPDATA%\Programs\deploy-dev e põe a pasta no PATH do usuário) e
// depois se atualiza sozinho a partir das releases.
var deployDev = Module{
	ID:    "deploy_dev",
	Grupo: GrupoDesenvolvimento,
	Title: "deploy-dev",
	Desc:  "CLI do deploy-tools que gera e valida o workflow de deploy dos projetos (github.com/vinicgobbi/deploy-tools)",
	Run: func(c *Ctx) error {
		c.Log.Infof("Instalando o deploy-dev (última release do deploy-tools)...")
		rel, err := ultimaRelease(c, "vinicgobbi/deploy-tools")
		if err != nil {
			return err
		}
		a, ok := rel.asset("deploy-dev-windows-amd64.exe")
		if !ok {
			return errorf("deploy-dev-windows-amd64.exe não está na release %s do deploy-tools", rel.Tag)
		}
		sum, err := rel.sha256(c, a)
		if err != nil {
			return err
		}
		exe := c.Temp(a.Name)
		if err := c.DownloadVerified(a.URL, exe, sum); err != nil {
			return err
		}
		if err := c.Run(exe, "install", "--add-to-path"); err != nil {
			return err
		}
		c.RefreshPath()
		c.Log.Successf("deploy-dev %s instalado.", rel.Tag)
		return nil
	},
}

// ---- Releases do GitHub ----

type releaseGitHub struct {
	Tag    string        `json:"tag_name"`
	Assets []assetGitHub `json:"assets"`
}

type assetGitHub struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
	// Digest é o SHA-256 que o GitHub calcula no upload ("sha256:<hex>").
	Digest string `json:"digest"`
}

func (a assetGitHub) sha256() string {
	s, _ := strings.CutPrefix(a.Digest, "sha256:")
	return s
}

// sha256 devolve o SHA-256 esperado de um asset: o digest que o GitHub
// calcula no upload ou, nas releases sem ele, o "<asset>.sha256" publicado
// junto (formato do sha256sum: "<hash>  <nome>").
func (r releaseGitHub) sha256(c *Ctx, a assetGitHub) (string, error) {
	if s := a.sha256(); s != "" {
		return s, nil
	}
	soma, ok := r.asset(a.Name + ".sha256")
	if !ok {
		return "", errorf("a release %s não publica o SHA-256 de %s", r.Tag, a.Name)
	}
	body, err := c.Get(soma.URL, 30*time.Second)
	if err != nil {
		return "", err
	}
	campos := strings.Fields(string(body))
	if len(campos) == 0 || len(campos[0]) != 64 {
		return "", errorf("%s inválido", soma.Name)
	}
	return campos[0], nil
}

func (r releaseGitHub) asset(nome string) (assetGitHub, bool) {
	for _, a := range r.Assets {
		if a.Name == nome {
			return a, true
		}
	}
	return assetGitHub{}, false
}

// ultimaRelease consulta a última release de um repositório do GitHub.
func ultimaRelease(c *Ctx, repo string) (releaseGitHub, error) {
	var rel releaseGitHub
	body, err := c.Get("https://api.github.com/repos/"+repo+"/releases/latest", time.Minute)
	if err != nil {
		return rel, err
	}
	return rel, json.Unmarshal(body, &rel)
}

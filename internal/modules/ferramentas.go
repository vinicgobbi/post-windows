package modules

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"strings"
	"time"

	"github.com/vinicgobbi/post-windows/internal/config"
)

var ferramentas = Module{
	ID:    "ferramentas",
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
		if err != nil {
			if c.Ctx.Err() != nil {
				return err
			}
			c.Log.Warnf("Não consegui instalar o pipx: %v", err)
		}
		c.Log.Successf("Ferramentas de terminal instaladas.")
		return nil
	},
}

var ruby = Module{
	ID:    "ruby",
	Title: "Instalar Ruby, Bundler e Jekyll",
	Desc:  "RubyInstaller com DevKit (MSYS2 + toolchain para gems nativas), Bundler e Jekyll; o Bundler instala as gems de cada projeto em vendor/bundle",
	Run: func(c *Ctx) error {
		// O instalador roda o "ridk install" ao final por padrão, num console
		// que espera Enter e trava o silent install; ele roda logo abaixo,
		// com a opção já escolhida.
		app := config.App{ID: "RubyInstallerTeam.RubyWithDevKit.3.4", Nome: "Ruby 3.4 (com DevKit)",
			Args: []string{"--override", "/VERYSILENT /NORESTART /ALLUSERS /MERGETASKS=!ridkinstall"}}
		if err := c.installApp(app); err != nil {
			return err
		}

		// Opção 3: MSYS2 atualizado + toolchain MinGW (o http_parser.rb,
		// dependência do "jekyll serve", compila na instalação).
		c.Log.Infof("Instalando a toolchain MSYS2/MinGW do Ruby (ridk install 3)...")
		if err := c.Run("ridk", "install", "3"); err != nil {
			if c.Ctx.Err() != nil {
				return err
			}
			c.Log.Warnf("ridk install falhou (%v); gems com extensão nativa podem não compilar.", err)
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

// O deploy-dev se instala sozinho (install copia para
// %LOCALAPPDATA%\Programs\deploy-dev e põe a pasta no PATH do usuário) e
// depois se atualiza sozinho a partir das releases.
var deployDev = Module{
	ID:    "deploy_dev",
	Title: "Instalar deploy-dev",
	Desc:  "CLI do deploy-tools que gera e valida o workflow de deploy dos projetos (github.com/vinicgobbi/deploy-tools)",
	Run: func(c *Ctx) error {
		c.Log.Infof("Instalando o deploy-dev (última release do deploy-tools)...")
		const asset = "deploy-dev-windows-amd64.exe"
		body, err := c.Get("https://api.github.com/repos/vinicgobbi/deploy-tools/releases/latest", time.Minute)
		if err != nil {
			return err
		}
		var rel struct {
			Assets []struct {
				Name string `json:"name"`
				URL  string `json:"browser_download_url"`
			} `json:"assets"`
		}
		if err := json.Unmarshal(body, &rel); err != nil {
			return err
		}
		urls := map[string]string{}
		for _, a := range rel.Assets {
			urls[a.Name] = a.URL
		}
		if urls[asset] == "" || urls[asset+".sha256"] == "" {
			return errorf("%s (ou o .sha256) não está na última release do deploy-tools", asset)
		}

		exe := c.Temp(asset)
		if err := c.Download(urls[asset], exe); err != nil {
			return err
		}
		if !c.DryRun {
			if err := conferirSHA256(c, urls[asset+".sha256"], exe); err != nil {
				return err
			}
		}
		if err := c.Run(exe, "install", "--add-to-path"); err != nil {
			return err
		}
		c.RefreshPath()
		c.Log.Successf("deploy-dev instalado.")
		return nil
	},
}

// conferirSHA256 compara o arquivo com o .sha256 publicado na release
// (formato do sha256sum: "<hash>  <nome>").
func conferirSHA256(c *Ctx, sumURL, arquivo string) error {
	body, err := c.Get(sumURL, 30*time.Second)
	if err != nil {
		return err
	}
	campos := strings.Fields(string(body))
	if len(campos) == 0 {
		return errorf("arquivo de SHA-256 vazio: %s", sumURL)
	}
	f, err := os.Open(arquivo)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	if real := hex.EncodeToString(h.Sum(nil)); !strings.EqualFold(real, campos[0]) {
		return errorf("SHA-256 de %s não confere (esperado %s, obtido %s)", arquivo, campos[0], real)
	}
	return nil
}

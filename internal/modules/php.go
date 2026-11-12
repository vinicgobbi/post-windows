package modules

import (
	"archive/zip"
	"bytes"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"maps"
	"os"
	"path"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/vinicgobbi/post-windows/internal/config"
	"github.com/vinicgobbi/post-windows/internal/winpath"
)

// phpVersao é a versão instalada pelo winget (pacote PHP.PHP.<versão>) e o
// sufixo das DLLs do driver SQL Server (php_sqlsrv_<85>_ts...).
const phpVersao = "8.5"

var phpSQLServer = Module{
	ID:    "php_sqlserver",
	Grupo: GrupoDesenvolvimento,
	Title: "PHP e SQL Server",
	Desc:  "PHP " + phpVersao + ", Composer (instalador conferido pelo SHA-384), driver ODBC 18 e as extensões sqlsrv/pdo_sqlsrv (DLLs oficiais da Microsoft, com SHA-256 conferido)",
	Run: func(c *Ctx) error {
		odbc := config.App{ID: "Microsoft.msodbcsql.18", Nome: "Microsoft ODBC Driver 18 for SQL Server"}
		if err := c.installApp(config.App{ID: "PHP.PHP." + phpVersao, Nome: "PHP " + phpVersao + " (thread-safe)"}); err != nil {
			return err
		}

		phpExe, err := c.LookPath("php")
		if err != nil && c.DryRun {
			// O PHP não foi instalado de verdade: segue com o caminho em
			// que o winget o deixaria, para mostrar o resto.
			phpExe, err = winpath.Join(c.Getenv("LOCALAPPDATA"), "Microsoft", "WinGet", "Links", "php.exe"), nil
		}
		if err != nil {
			c.Log.Warnf("php.exe não encontrado no PATH após a instalação; pulando Composer e as extensões sqlsrv.")
			return c.installApp(odbc)
		}
		// O winget instala o PHP como pacote "portable" e cria um link em
		// %LOCALAPPDATA%\Microsoft\WinGet\Links\php.exe; o alvo real do
		// link é a pasta de instalação (onde ficam ext\ e o php.ini).
		if real, err := c.EvalSymlinks(phpExe); err == nil {
			phpExe = real
		}
		phpDir := winpath.Dir(phpExe)
		confDir := phpConfDir(c)
		ini := winpath.Join(confDir, "php.ini")

		if err := prepararPhpIni(c, phpDir, confDir); err != nil {
			return err
		}
		if err := instalarComposer(c, phpExe); err != nil {
			return err
		}
		if err := c.installApp(odbc); err != nil {
			return err
		}
		if err := instalarSqlsrv(c, winpath.Join(confDir, "ext"), ini); err != nil {
			return err
		}
		conferirPhpIni(c, phpExe, ini)
		c.Log.Successf("PHP + sqlsrv/pdo_sqlsrv configurados (php.ini em %s).", confDir)
		return nil
	},
}

// phpConfDir é a pasta fixa do php.ini e das DLLs que não vêm com o PHP
// (sqlsrv). Fica fora da pasta do pacote porque o winget recria a pasta do
// PHP a cada atualização, e o php.ini (e o que mais tiver sido posto ali)
// some ou fica para trás. É por versão (8.5) porque as DLLs de extensão só
// servem para a versão do PHP para a qual foram compiladas.
func phpConfDir(c *Ctx) string {
	return winpath.Join(c.Getenv("ProgramData"), "PHP", phpVersao)
}

// chavePHP é lida pelo próprio PHP para achar o php.ini, antes da pasta do
// php.exe (php_ini.c/win32/registry.c): o valor IniFilePath de
// HKLM\SOFTWARE\PHP\<maior>.<menor>. Por ser por versão, não afeta outro PHP
// instalado lado a lado (ao contrário da variável PHPRC, que vale para
// todos).
const chavePHP = `HKLM\SOFTWARE\PHP\` + phpVersao

// prepararPhpIni monta o php.ini em confDir e aponta o PHP para ele pelo
// registro. A base é, em ordem: o php.ini que já esteja em confDir (as
// reexecuções preservam o que foi editado à mão), o php.ini antigo da pasta
// do pacote (migração) ou o php.ini-production (o pacote do winget não traz
// um php.ini ativo, ou seja, sem nenhuma extensão). Habilita
// openssl/curl/mbstring, que o Composer precisa para baixar via HTTPS — sem
// elas o instalador falha em silêncio, sem gerar o composer.phar.
func prepararPhpIni(c *Ctx, phpDir, confDir string) error {
	ini := winpath.Join(confDir, "php.ini")
	var base []byte
	for _, p := range []string{ini, winpath.Join(phpDir, "php.ini"), winpath.Join(phpDir, "php.ini-production")} {
		if data, err := c.ReadFile(p); err == nil {
			if p != ini {
				c.Log.Infof("Criando %s a partir de %s.", ini, p)
			}
			base = data
			break
		}
	}
	if base == nil && c.DryRun {
		// O PHP não foi instalado de verdade: o php.ini-production viria
		// com ele.
		c.Log.Infof("[dry-run] criar %s a partir do php.ini-production do PHP", ini)
		return c.SetString(chavePHP, "IniFilePath", confDir)
	}
	if base == nil {
		c.Log.Warnf("Nem php.ini nem php.ini-production em %s; Composer e sqlsrv podem falhar.", phpDir)
		return nil
	}

	novo := habilitarExtensoes(string(base), "openssl", "curl", "mbstring")
	if atual, err := c.ReadFile(ini); err != nil || string(atual) != novo {
		if err := c.WriteFile(ini, novo); err != nil {
			return err
		}
	}
	return c.SetString(chavePHP, "IniFilePath", confDir)
}

var (
	reExtDirComentado = regexp.MustCompile(`(?m)^;[ \t]*extension_dir[ \t]*=[ \t]*"ext"[ \t]*(\r?)$`)
	// extension_dir com caminho absoluto (ex.: herdado do php.ini antigo,
	// apontando para a pasta de uma versão anterior do pacote).
	reExtDirAbsoluto = regexp.MustCompile(`(?m)^[ \t]*extension_dir[ \t]*=[ \t]*"?(?:[A-Za-z]:)?[\\/][^\r\n]*?(\r?)$`)
)

// habilitarExtensoes descomenta "extension=X" e garante extension_dir =
// "ext". O "ext" relativo é o que mantém as extensões que vêm com o PHP
// funcionando depois que o winget muda a pasta do pacote: o PHP passa o
// caminho "ext\php_X.dll" direto ao LoadLibrary, que procura primeiro na
// pasta do executável (php.exe), e não na do php.ini. Só mexe em linhas
// comentadas e em extension_dir absoluto: não sobrescreve o resto do que foi
// configurado à mão.
func habilitarExtensoes(ini string, exts ...string) string {
	for _, e := range exts {
		re := regexp.MustCompile(`(?m)^;[ \t]*extension[ \t]*=[ \t]*` + regexp.QuoteMeta(e) + `[ \t]*(\r?)$`)
		ini = re.ReplaceAllString(ini, "extension="+e+"$1")
	}
	ini = reExtDirAbsoluto.ReplaceAllString(ini, `extension_dir = "ext"$1`)
	return reExtDirComentado.ReplaceAllString(ini, `extension_dir = "ext"$1`)
}

// definirExtensao troca qualquer "extension=" que carregue a DLL nome (com
// ou sem caminho, comentado ou não) por uma linha só, com o caminho
// absoluto — o PHP usa um caminho com barra como está, sem passar pelo
// extension_dir (ext/standard/dl.c).
func definirExtensao(ini, nome, caminho string) string {
	re := regexp.MustCompile(`(?mi)^[ \t;]*extension[ \t]*=[^\r\n]*` + regexp.QuoteMeta(nome) + `[^\r\n]*\r?\n?`)
	ini = re.ReplaceAllString(ini, "")
	nl := "\n"
	if strings.Contains(ini, "\r\n") {
		nl = "\r\n"
	}
	if ini != "" && !strings.HasSuffix(ini, "\n") {
		ini += nl
	}
	return ini + "extension=" + caminho + nl
}

// conferirPhpIni confirma, com o próprio php, que o php.ini carregado é o da
// pasta fixa e que as extensões carregam.
func conferirPhpIni(c *Ctx, phpExe, ini string) {
	if c.DryRun {
		return
	}
	out, err := c.Output(phpExe, "-r", `echo php_ini_loaded_file(), "|", implode(",", array_filter(["openssl","curl","mbstring","sqlsrv","pdo_sqlsrv"], "extension_loaded"));`)
	if err != nil {
		c.Log.Warnf("Não consegui conferir a configuração do PHP: %v", err)
		return
	}
	carregado, exts, _ := strings.Cut(strings.TrimSpace(out), "|")
	if !winpath.Igual(carregado, ini) {
		c.Log.Warnf("O PHP carregou %q em vez de %s; confira o valor IniFilePath em %s.", carregado, ini, chavePHP)
		return
	}
	c.Log.Infof("php.ini em uso: %s (extensões carregadas: %s).", carregado, exts)
}

// instalarComposer segue o "command-line installation" de
// getcomposer.org/download: baixa o composer-setup.php, confere o SHA-384
// publicado pelo projeto antes de rodar e gera o composer.phar. Como .phar
// não é executável direto no Windows, cria um composer.bat ao lado.
func instalarComposer(c *Ctx, phpExe string) error {
	if c.Exists("composer") {
		c.Log.Successf("Composer já está instalado.")
		return nil
	}
	c.Log.Infof("Instalando o Composer...")
	dir := winpath.Join(c.Getenv("ProgramData"), "Composer")
	setup := c.Temp("composer-setup.php")
	if err := c.Download("https://getcomposer.org/installer", setup); err != nil {
		return err
	}
	if c.DryRun {
		c.Log.Infof("[dry-run] %s %s --install-dir=%s", phpExe, setup, dir)
		return nil
	}

	sig, err := c.Get("https://composer.github.io/installer.sig", 30*time.Second)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(setup)
	if err != nil {
		return err
	}
	soma := sha512.Sum384(data)
	if hex.EncodeToString(soma[:]) != strings.TrimSpace(string(sig)) {
		c.Log.Warnf("Assinatura do instalador do Composer não confere; abortando por segurança.")
		return nil
	}

	if err := step(
		func() error { return c.MkdirAll(dir) },
		func() error { return c.Run(phpExe, setup, "--install-dir="+dir, "--quiet") },
	); err != nil {
		return err
	}
	if !c.FileExists(winpath.Join(dir, "composer.phar")) {
		c.Log.Warnf("composer.phar não foi gerado; a instalação do Composer falhou.")
		return nil
	}
	if err := step(
		func() error {
			return c.WriteFile(winpath.Join(dir, "composer.bat"), "@php \"%~dp0composer.phar\" %*\r\n")
		},
		func() error { return c.AddToMachinePath(dir) },
	); err != nil {
		return err
	}
	c.RefreshPath()
	c.Log.Successf("Composer instalado em %s.", dir)
	return nil
}

var reDllSqlsrv = regexp.MustCompile(`(?i)^php_(pdo_)?sqlsrv_` + strings.ReplaceAll(phpVersao, ".", "") + `_ts(_x64)?\.dll$`)

// instalarSqlsrv baixa a release mais recente do driver oficial
// (microsoft/msphpsql — no Windows não dá para compilar via PECL como no
// Linux), conferida pelo SHA-256 que o GitHub publica, copia as DLLs
// thread-safe x64 da versão do PHP para extDir (na pasta fixa, para
// sobreviverem às atualizações do pacote) e as habilita no php.ini pelo
// caminho absoluto.
func instalarSqlsrv(c *Ctx, extDir, ini string) error {
	c.Log.Infof("Baixando o driver PHP para SQL Server (msphpsql) mais recente...")
	rel, err := ultimaRelease(c, "microsoft/msphpsql")
	if err != nil {
		return err
	}
	var pacote assetGitHub
	for _, a := range rel.Assets {
		if strings.HasPrefix(a.Name, "Windows") && strings.HasSuffix(a.Name, ".zip") {
			pacote = a
			break
		}
	}
	if pacote.URL == "" {
		c.Log.Warnf("Não encontrei o pacote Windows na release %s do msphpsql; pulando as extensões sqlsrv.", rel.Tag)
		return nil
	}
	if pacote.sha256() == "" {
		return errorf("o GitHub não publicou o SHA-256 de %s (release %s do msphpsql)", pacote.Name, rel.Tag)
	}

	zipPath := c.Temp("msphpsql.zip")
	if err := c.DownloadVerified(pacote.URL, zipPath, pacote.sha256()); err != nil {
		return err
	}
	if c.DryRun {
		c.Log.Infof("[dry-run] copiar php_sqlsrv/php_pdo_sqlsrv %s para %s e habilitar no php.ini", phpVersao, extDir)
		return nil
	}
	dlls, err := dllsSqlsrv(zipPath)
	if err != nil {
		return err
	}
	if len(dlls) < 2 {
		c.Log.Warnf("As DLLs sqlsrv/pdo_sqlsrv do PHP %s (TS x64) não estão na release %s (a versão pode ainda não ter suporte); pulando.", phpVersao, rel.Tag)
		return nil
	}
	conteudo, err := c.ReadFile(ini)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	novo := string(conteudo)
	for _, nome := range slices.Sorted(maps.Keys(dlls)) {
		destino := winpath.Join(extDir, nome)
		if err := c.WriteFile(destino, string(dlls[nome])); err != nil {
			return err
		}
		novo = definirExtensao(novo, nome, destino)
	}
	return c.WriteFile(ini, novo)
}

// dllsSqlsrv procura, no zip do msphpsql (e em zips dentro dele), as DLLs
// thread-safe x64 da versão do PHP. Devolve nome → conteúdo.
func dllsSqlsrv(zipPath string) (map[string][]byte, error) {
	data, err := os.ReadFile(zipPath)
	if err != nil {
		return nil, err
	}
	achadas := map[string][]byte{}
	return achadas, procurarDlls(data, achadas, 0)
}

func procurarDlls(data []byte, achadas map[string][]byte, nivel int) error {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}
	for _, f := range zr.File {
		nome := path.Base(f.Name)
		ehZip := strings.EqualFold(path.Ext(nome), ".zip") && nivel == 0
		// Prefere a pasta x64 quando o nome da DLL não diz a arquitetura.
		if !ehZip && (!reDllSqlsrv.MatchString(nome) || strings.Contains(strings.ToLower(f.Name), "x86")) {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		conteudo, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return err
		}
		if ehZip {
			if err := procurarDlls(conteudo, achadas, nivel+1); err != nil {
				return err
			}
			continue
		}
		achadas[nome] = conteudo
	}
	return nil
}

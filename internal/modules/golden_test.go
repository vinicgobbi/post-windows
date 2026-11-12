package modules

// Testes golden do --dry-run: cada cenário descreve uma máquina Windows (o
// que está instalado, os arquivos, o registro, as variáveis de ambiente, o
// que os scripts do PowerShell e a rede respondem) e o teste roda todos os
// módulos em dry-run contra ela, comparando o que cada um faria com
// testdata/golden/<cenário>.txt. Qualquer mudança no que um módulo faz
// aparece como diferença nesses arquivos. Os caminhos usam o pacote
// winpath, então o resultado é o mesmo no Linux e no Windows.
//
// Para regenerar depois de uma mudança intencional (e revisar o diff):
//
//	go test ./internal/modules -run TestGolden -update

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/vinicgobbi/post-windows/internal/runlog"
	"github.com/vinicgobbi/post-windows/internal/runner"
	"github.com/vinicgobbi/post-windows/internal/sysinfo"
)

var atualizarGolden = flag.Bool("update", false, "regrava os arquivos de testdata/golden")

// ---- Máquina falsa ----

type hostFalso struct {
	// arquivos usa caminhos com "/" e a unidade: C:\Users\ana -> C:/Users/ana.
	arquivos fstest.MapFS
	binarios map[string]bool
	env      map[string]string
	// registro: chave -> nomes dos valores.
	registro map[string][]string
	// comandos: "nome args" -> stdout. Ausente = o comando falha (código
	// de saída 1).
	comandos map[string]string
	// powershell: trecho do script -> stdout. Vale o primeiro trecho (em
	// ordem alfabética) contido no script; nenhum = o script falha.
	powershell map[string]string
	// http: URL -> corpo (HTTP 200). Ausente = HTTP 404.
	http map[string]string

	mu          sync.Mutex
	semResposta map[string]bool
}

func (h *hostFalso) anotar(s string) {
	h.mu.Lock()
	if h.semResposta == nil {
		h.semResposta = map[string]bool{}
	}
	h.semResposta[s] = true
	h.mu.Unlock()
}

func (h *hostFalso) Consultar(name string, args []string) ([]byte, []byte, error) {
	if name == "powershell" {
		script := args[0]
		trechos := make([]string, 0, len(h.powershell))
		for t := range h.powershell {
			trechos = append(trechos, t)
		}
		sort.Strings(trechos)
		for _, t := range trechos {
			if strings.Contains(script, t) {
				return []byte(h.powershell[t]), nil, nil
			}
		}
		h.anotar("powershell: " + primeiraLinha(script))
		return nil, nil, errors.New("exit status 1")
	}
	k := strings.Join(append([]string{name}, args...), " ")
	if out, ok := h.comandos[k]; ok {
		return []byte(out), nil, nil
	}
	h.anotar("comando: " + k)
	return nil, nil, errors.New("exit status 1")
}

func primeiraLinha(s string) string {
	for _, l := range strings.Split(s, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			return l
		}
	}
	return ""
}

func (h *hostFalso) LookPath(name string) (string, error) {
	if h.binarios[name] {
		return `C:\bin\` + name + ".exe", nil
	}
	return "", fmt.Errorf("exec: %q: executable file not found in %%PATH%%", name)
}

func (h *hostFalso) Getenv(name string) string { return h.env[name] }

func (h *hostFalso) ValueNames(key string) ([]string, error) {
	if v, ok := h.registro[key]; ok {
		return v, nil
	}
	return nil, fs.ErrNotExist
}

func (h *hostFalso) FS() fs.FS {
	if h.arquivos == nil {
		h.arquivos = fstest.MapFS{}
	}
	return h.arquivos
}

func (h *hostFalso) Transport() http.RoundTripper { return h }

func (h *hostFalso) RoundTrip(req *http.Request) (*http.Response, error) {
	u := req.URL.String()
	corpo, ok := h.http[u]
	status := http.StatusOK
	if !ok {
		h.anotar("GET " + u)
		status, corpo = http.StatusNotFound, "não encontrado"
	}
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(corpo)),
		Header:     http.Header{},
		Request:    req,
	}, nil
}

// ctxTeste monta um Ctx em dry-run contra a máquina falsa, com a saída do
// log descartada.
func ctxTeste(t *testing.T, h *hostFalso) *Ctx {
	t.Helper()
	log, err := runlog.New("", "")
	if err != nil {
		t.Fatal(err)
	}
	return &Ctx{
		Runner: &runner.Runner{Ctx: context.Background(), Log: log, DryRun: true, WorkDir: workDir, Host: h},
		Sys:    &sysinfo.Info{Edition: "Windows 11 Pro", Build: 26200, Arch: "amd64"},
	}
}

// ---- Cenários ----

type cenario struct {
	nome      string
	sys       sysinfo.Info
	host      *hostFalso
	wallpaper string
}

const (
	dirPacote = `C:\post-windows`
	workDir   = `C:\Users\ana\AppData\Local\Temp\post-windows-teste`
	sha       = "0000000000000000000000000000000000000000000000000000000000000000"
)

func arq(s string) *fstest.MapFile { return &fstest.MapFile{Data: []byte(s), Mode: 0o644} }

// fsPath converte um caminho do Windows para a chave do fstest.MapFS.
func fsPath(p string) string { return strings.ReplaceAll(p, `\`, "/") }

// exportWinget é o JSON do "winget export" com os IDs dados.
func exportWinget(ids ...string) string {
	pkgs := make([]string, len(ids))
	for i, id := range ids {
		pkgs[i] = `{"PackageIdentifier":"` + id + `"}`
	}
	return "\ufeff" + `{"Sources":[{"Packages":[` + strings.Join(pkgs, ",") + `],"SourceDetails":{"Name":"winget"}}]}`
}

// windowsLimpo é um Windows 11 Pro 25H2 recém-instalado, num desktop com
// VT-x ligado: só o que vem com o sistema (winget, Edge), sem Git, PHP nem
// Chocolatey, com a letra D: livre e rede.
func windowsLimpo() cenario {
	env := map[string]string{
		"USERPROFILE":       `C:\Users\ana`,
		"USERNAME":          "ana",
		"APPDATA":           `C:\Users\ana\AppData\Roaming`,
		"LOCALAPPDATA":      `C:\Users\ana\AppData\Local`,
		"TEMP":              `C:\Users\ana\AppData\Local\Temp`,
		"ProgramData":       `C:\ProgramData`,
		"ProgramFiles":      `C:\Program Files`,
		"ProgramFiles(x86)": `C:\Program Files (x86)`,
		"SystemDrive":       "C:",
		"WINDIR":            `C:\Windows`,
	}
	h := &hostFalso{
		env: env,
		arquivos: fstest.MapFS{
			fsPath(workDir + `\winget-export.json`):                                              arq(exportWinget("Microsoft.Edge", "Microsoft.AppInstaller")),
			fsPath(`C:\Users\ana\AppData\Local\Temp\arquivo.tmp`):                                arq("x"),
			fsPath(`C:\Program Files\WindowsApps\Microsoft.DesktopAppInstaller_1.27\winget.exe`): arq("MZ"),
		},
		binarios: map[string]bool{"winget": true},
		registro: map[string][]string{
			keyRun: {"OneDrive", "MicrosoftEdgeAutoLaunch_0123ABCD"},
		},
		comandos: map[string]string{
			"winget export -o " + workDir + `\winget-export.json --accept-source-agreements`: "",
			"sc.exe query dmwappushsvc": "SERVICE_NAME: dmwappushsvc\r\n        STATE              : 1  STOPPED\r\n",
			"sc.exe qc DiagTrack":       "SERVICE_NAME: DiagTrack\r\n        START_TYPE         : 2   AUTO_START\r\n",
			"fsutil devdrv query D:":    "Este volume é um Dev Drive confiável.\r\n",
			`schtasks /Query /TN \Microsoft\Windows\Application Experience\Microsoft Compatibility Appraiser`: "ok",
			`schtasks /Query /TN \Microsoft\Windows\Customer Experience Improvement Program\Consolidator`:     "ok",
		},
		powershell: map[string]string{
			"Get-AppxPackage -Name Microsoft.DesktopAppInstaller": `C:\Program Files\WindowsApps\Microsoft.DesktopAppInstaller_1.27`,
			"Get-WindowsUpdate -AcceptAll":                        "SEMMODULO",
			"$v = Get-Volume -DriveLetter D":                      "LIVRE",
			"WindowsIdentity]::GetCurrent().User.Value":           "S-1-5-21-1-2-3-1001",
			"GetFolderPath('MyDocuments')":                        `C:\Users\ana\Documents`,
			"VirtualizationFirmwareEnabled":                       "True",
			"Microsoft-Hyper-V-All":                               "Disabled",
			"[bool](Get-CimInstance -ClassName Win32_Battery":     "False",
		},
		http: map[string]string{
			"https://api.github.com/repos/vinicgobbi/deploy-tools/releases/latest": `{"tag_name":"v1.0.0","assets":[
				{"name":"deploy-dev-windows-amd64.exe","browser_download_url":"https://github.com/vinicgobbi/deploy-tools/releases/download/v1.0.0/deploy-dev-windows-amd64.exe","digest":"sha256:` + sha + `"},
				{"name":"deploy-dev-windows-amd64.exe.sha256","browser_download_url":"https://github.com/vinicgobbi/deploy-tools/releases/download/v1.0.0/deploy-dev-windows-amd64.exe.sha256","digest":""}]}`,
			"https://api.github.com/repos/microsoft/msphpsql/releases/latest": `{"tag_name":"v5.13.0","assets":[
				{"name":"Linux_5.13.0.tar","browser_download_url":"https://github.com/microsoft/msphpsql/releases/download/v5.13.0/Linux_5.13.0.tar","digest":"sha256:` + sha + `"},
				{"name":"Windows_5.13.0.zip","browser_download_url":"https://github.com/microsoft/msphpsql/releases/download/v5.13.0/Windows_5.13.0.zip","digest":"sha256:` + sha + `"}]}`,
		},
	}
	return cenario{
		nome:      "windows-11-limpo",
		sys:       sysinfo.Info{Edition: "Windows 11 Pro", EditionID: "Professional", DisplayVersion: "25H2", Build: 26200, UBR: 7171, Arch: "amd64"},
		host:      h,
		wallpaper: dirPacote + `\Wallpapers\paisagem.jpg`,
	}
}

// windowsConfigurado é um Windows 11 onde uma versão anterior do
// post-windows já rodou: apps instalados (inclusive o fnm, com a ativação
// nos perfis), Dev Drive montado, Rust em ~\.cargo, PHP com php.ini na pasta
// fixa, Chocolatey, perfis OpenVPN, e os casos menos comuns: notebook na
// bateria, Hyper-V ligado, VT-x desligado, DiagTrack desativado e o
// deploy-tools sem o digest do GitHub (só o .sha256).
func windowsConfigurado() cenario {
	c := windowsLimpo()
	c.nome = "windows-11-configurado"
	c.wallpaper = ""
	h := c.host
	for _, b := range []string{"git", "oh-my-posh", "choco", "php", "composer", "mise"} {
		h.binarios[b] = true
	}
	h.arquivos[fsPath(workDir+`\winget-export.json`)] = arq(exportWinget(
		"Microsoft.Edge", "Google.Chrome", "Mozilla.Firefox", "Git.Git", "Git.GCM",
		"Microsoft.VisualStudioCode", "Schniz.fnm", "Rustlang.Rustup", "Spotify.Spotify",
		"PHP.PHP.8.5", "Microsoft.PowerShell", "JanDeDobbeleer.OhMyPosh"))
	h.arquivos[fsPath(`C:\Users\ana\.cargo\bin\cargo.exe`)] = arq("MZ")
	h.arquivos[fsPath(`C:\Users\ana\Projects\site\index.html`)] = arq("<html>")
	h.arquivos[fsPath(`D:\Projects\api\main.go`)] = arq("package main")
	h.arquivos[fsPath(`C:\ProgramData\PHP\8.5\php.ini`)] = arq("[PHP]\r\nextension=curl\r\nextension=openssl\r\nextension=mbstring\r\nextension_dir = \"ext\"\r\n")
	h.arquivos[fsPath(`C:\Users\ana\Documents\PowerShell\Microsoft.PowerShell_profile.ps1`)] = arq(linhaFnm + "\r\n. \"C:\\Users\\ana\\.config\\oh-my-posh\\profile.ps1\"\r\n")
	h.arquivos[fsPath(`C:\Users\ana\Documents\WindowsPowerShell\Microsoft.PowerShell_profile.ps1`)] = arq(linhaFnm + "\r\n")
	h.arquivos[fsPath(`C:\Program Files (x86)\Microsoft Visual Studio\Installer\vswhere.exe`)] = arq("MZ")
	h.arquivos[fsPath(dirPacote+`\OVPN\empresa.ovpn`)] = arq("client\nremote vpn.exemplo.test 1194\n")
	h.arquivos[fsPath(dirPacote+`\OVPN\escritorio.ovpn`)] = arq("client\n")
	h.arquivos[fsPath(dirPacote+`\OVPN\README.md`)] = arq("# Perfis\n")
	h.env["CARGO_HOME"] = ""
	h.comandos[`php -r echo php_ini_loaded_file(), "|", implode(",", array_filter(["openssl","curl","mbstring","sqlsrv","pdo_sqlsrv"], "extension_loaded"));`] = `C:\ProgramData\PHP\8.5\php.ini|openssl,curl,mbstring`
	h.comandos["sc.exe qc DiagTrack"] = "SERVICE_NAME: DiagTrack\r\n        START_TYPE         : 4   DISABLED\r\n"
	delete(h.comandos, "sc.exe query dmwappushsvc")
	h.powershell["$v = Get-Volume -DriveLetter D"] = "REFS"
	h.powershell["(Get-Volume -DriveLetter D).FileSystem"] = "ReFS"
	h.powershell["VirtualizationFirmwareEnabled"] = "False"
	h.powershell["Microsoft-Hyper-V-All"] = "Enabled"
	h.powershell["[bool](Get-CimInstance -ClassName Win32_Battery"] = "True"
	h.powershell["BatteryStatus"] = "1"
	h.http["https://api.github.com/repos/vinicgobbi/deploy-tools/releases/latest"] = `{"tag_name":"v1.1.0","assets":[
		{"name":"deploy-dev-windows-amd64.exe","browser_download_url":"https://github.com/vinicgobbi/deploy-tools/releases/download/v1.1.0/deploy-dev-windows-amd64.exe"},
		{"name":"deploy-dev-windows-amd64.exe.sha256","browser_download_url":"https://github.com/vinicgobbi/deploy-tools/releases/download/v1.1.0/deploy-dev-windows-amd64.exe.sha256"}]}`
	h.http["https://github.com/vinicgobbi/deploy-tools/releases/download/v1.1.0/deploy-dev-windows-amd64.exe.sha256"] = sha + "  deploy-dev-windows-amd64.exe\n"
	return c
}

// windows10 é um Windows 10 22H2 Home recém-instalado: sem Dev Drive (só
// existe no Windows 11) e com a letra D: ocupada por um pendrive.
func windows10() cenario {
	c := windowsLimpo()
	c.nome = "windows-10-limpo"
	c.sys = sysinfo.Info{Edition: "Windows 10 Home", EditionID: "Core", DisplayVersion: "22H2", Build: 19045, UBR: 6456, Arch: "amd64"}
	c.host.powershell["$v = Get-Volume -DriveLetter D"] = "OCUPADA Removable FAT32"
	return c
}

func cenarios() []cenario {
	return []cenario{windowsLimpo(), windowsConfigurado(), windows10()}
}

// ---- Execução ----

// normalizar troca valores que mudam sem alterar o comportamento dos
// módulos (commits fixados, atualizados pelo workflow pins.yml) por nomes
// fixos, para esses PRs não exigirem regravar os golden.
func normalizar(s string) string {
	return strings.NewReplacer(dotfilesCommit, "<dotfilesCommit>").Replace(s)
}

func rodarCenario(t *testing.T, c cenario) string {
	var (
		out strings.Builder
		mu  sync.Mutex
	)
	log, err := runlog.New("", "")
	if err != nil {
		t.Fatal(err)
	}
	log.SetSink(func(e runlog.Entry) {
		mu.Lock()
		fmt.Fprintf(&out, "[%s] %s\n", e.Level, e.Text)
		mu.Unlock()
	})
	sys := c.sys
	ctx := &Ctx{
		Runner: &runner.Runner{
			Ctx:     context.Background(),
			Log:     log,
			DryRun:  true,
			WorkDir: workDir,
			Host:    c.host,
		},
		Sys:            &sys,
		OVPNDir:        dirPacote + `\OVPN`,
		DevDriveLetter: "D",
		DevDriveGB:     150,
		Wallpaper:      c.wallpaper,
		Agora:          func() time.Time { return time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC) },
	}

	fmt.Fprintf(&out, "# Cenário %s: %s, dry-run\n", c.nome, sys.Summary())
	for _, m := range All() {
		mu.Lock()
		fmt.Fprintf(&out, "\n== %s (%s)\n", m.ID, m.Title)
		mu.Unlock()
		err := m.Run(ctx)
		mu.Lock()
		if err != nil {
			fmt.Fprintf(&out, "=> ERRO: %v\n", err)
		}
		mu.Unlock()
	}

	pend := make([]string, 0, len(c.host.semResposta))
	for k := range c.host.semResposta {
		pend = append(pend, k)
	}
	slices.Sort(pend)
	out.WriteString("\n== Consultas que o cenário não responde (falham na máquina falsa)\n")
	for _, p := range pend {
		out.WriteString(p + "\n")
	}
	return normalizar(out.String())
}

func TestGolden(t *testing.T) {
	for _, c := range cenarios() {
		t.Run(c.nome, func(t *testing.T) {
			got := rodarCenario(t, c)
			arquivo := filepath.Join("testdata", "golden", c.nome+".txt")
			if *atualizarGolden {
				if err := os.MkdirAll(filepath.Dir(arquivo), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(arquivo, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(arquivo)
			if err != nil {
				t.Fatalf("%v (rode com -update para criar)", err)
			}
			// O Git no Windows pode converter o arquivo para CRLF.
			if w := strings.ReplaceAll(string(want), "\r\n", "\n"); got != w {
				t.Errorf("o dry-run do cenário %s mudou em relação a %s.\n"+
					"Se a mudança for intencional, rode: go test ./internal/modules -run TestGolden -update\n%s",
					c.nome, arquivo, diffLinhas(w, got))
			}
		})
	}
}

// diffLinhas mostra as primeiras linhas que diferem (sem dependências
// externas: basta para localizar a mudança).
func diffLinhas(want, got string) string {
	a, b := strings.Split(want, "\n"), strings.Split(got, "\n")
	var out strings.Builder
	n := 0
	for i := 0; i < max(len(a), len(b)) && n < 20; i++ {
		var x, y string
		if i < len(a) {
			x = a[i]
		}
		if i < len(b) {
			y = b[i]
		}
		if x != y {
			fmt.Fprintf(&out, "linha %d:\n  - %s\n  + %s\n", i+1, x, y)
			n++
		}
	}
	return out.String()
}

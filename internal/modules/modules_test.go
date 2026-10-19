package modules

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestIDsUnicos(t *testing.T) {
	vistos := map[string]bool{}
	for _, m := range All() {
		if vistos[m.ID] {
			t.Errorf("ID duplicado: %s", m.ID)
		}
		vistos[m.ID] = true
		if m.Run == nil || m.Title == "" || m.Desc == "" {
			t.Errorf("%s incompleto (Run/Title/Desc)", m.ID)
		}
	}
}

func TestParseWingetExport(t *testing.T) {
	data := []byte("\ufeff" + `{"$schema":"x","Sources":[
		{"Packages":[{"PackageIdentifier":"Git.Git"},{"PackageIdentifier":"Google.Chrome"}],"SourceDetails":{"Name":"winget"}},
		{"Packages":[{"PackageIdentifier":"9PGCV4V3BK4W"}],"SourceDetails":{"Name":"msstore"}}]}`)
	ids, err := parseWingetExport(data)
	if err != nil || !slices.Equal(ids, []string{"Git.Git", "Google.Chrome", "9PGCV4V3BK4W"}) {
		t.Errorf("ids = %v, err = %v", ids, err)
	}
}

func TestHabilitarExtensoes(t *testing.T) {
	ini := ";extension=curl\n;extension=openssl\n; extension = mbstring\n;extension=gd\nextension=zip\n;extension_dir = \"ext\"\n"
	got := habilitarExtensoes(ini, "openssl", "curl", "mbstring")
	want := "extension=curl\nextension=openssl\nextension=mbstring\n;extension=gd\nextension=zip\nextension_dir = \"ext\"\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	// CRLF (o php.ini-production do Windows) é preservado.
	if got := habilitarExtensoes(";extension=curl\r\n;extension=gd\r\n", "curl"); got != "extension=curl\r\n;extension=gd\r\n" {
		t.Errorf("CRLF: %q", got)
	}
	// Idempotente.
	if again := habilitarExtensoes(got, "openssl", "curl", "mbstring"); again != got {
		t.Error("segunda passada alterou o ini")
	}
}

func zipCom(t *testing.T, arquivos map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for nome, data := range arquivos {
		w, err := zw.Create(nome)
		if err != nil {
			t.Fatal(err)
		}
		w.Write(data)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestDllsSqlsrv(t *testing.T) {
	interno := zipCom(t, map[string][]byte{
		"x64/php_sqlsrv_85_ts.dll":     []byte("ts64"),
		"x64/php_sqlsrv_85_nts.dll":    []byte("nts"),
		"x86/php_pdo_sqlsrv_85_ts.dll": []byte("x86"),
	})
	externo := zipCom(t, map[string][]byte{
		"Windows-8.5/x64.zip":                          interno,
		"Windows-8.5/x64/php_pdo_sqlsrv_85_ts_x64.dll": []byte("pdo64"),
		"Windows-8.4/x64/php_sqlsrv_84_ts_x64.dll":     []byte("84"),
	})
	p := filepath.Join(t.TempDir(), "m.zip")
	os.WriteFile(p, externo, 0o644)

	dlls, err := dllsSqlsrv(p)
	if err != nil {
		t.Fatal(err)
	}
	var nomes []string
	for n := range dlls {
		nomes = append(nomes, n)
	}
	slices.Sort(nomes)
	if !slices.Equal(nomes, []string{"php_pdo_sqlsrv_85_ts_x64.dll", "php_sqlsrv_85_ts.dll"}) {
		t.Errorf("dlls = %v", nomes)
	}
}

func TestHabilitarExtensoesExtDirAbsoluto(t *testing.T) {
	// php.ini antigo migrado da pasta de uma versão anterior do pacote.
	ini := "extension_dir = \"C:\\Users\\x\\AppData\\Local\\Microsoft\\WinGet\\Packages\\PHP.PHP.8.5_old\\ext\"\r\nextension=curl\r\n"
	got := habilitarExtensoes(ini)
	if got != "extension_dir = \"ext\"\r\nextension=curl\r\n" {
		t.Errorf("got %q", got)
	}
	// Um extension_dir relativo já configurado fica como está.
	if got := habilitarExtensoes("extension_dir = \"ext\"\n"); got != "extension_dir = \"ext\"\n" {
		t.Errorf("relativo: %q", got)
	}
}

func TestDefinirExtensao(t *testing.T) {
	ini := "[PHP]\r\nextension=curl\r\nextension=php_sqlsrv_85_ts.dll\r\n;extension=C:\\velho\\ext\\php_sqlsrv_85_ts.dll\r\nextension=php_pdo_sqlsrv_85_ts.dll\r\n"
	got := definirExtensao(ini, "php_sqlsrv_85_ts.dll", `C:\ProgramData\PHP\8.5\ext\php_sqlsrv_85_ts.dll`)
	want := "[PHP]\r\nextension=curl\r\nextension=php_pdo_sqlsrv_85_ts.dll\r\nextension=C:\\ProgramData\\PHP\\8.5\\ext\\php_sqlsrv_85_ts.dll\r\n"
	if got != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}
	// Idempotente: rodar de novo não duplica a linha.
	if again := definirExtensao(got, "php_sqlsrv_85_ts.dll", `C:\ProgramData\PHP\8.5\ext\php_sqlsrv_85_ts.dll`); again != got {
		t.Errorf("segunda passada: %q", again)
	}
	// Não confunde php_sqlsrv com php_pdo_sqlsrv.
	if !strings.Contains(got, "extension=php_pdo_sqlsrv_85_ts.dll") {
		t.Error("removeu a linha do pdo_sqlsrv")
	}
}

func TestStartupDesativado(t *testing.T) {
	v := startupDesativado(time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC))
	if len(v) != 12 || v[0] != 0x03 || v[1] != 0 || v[2] != 0 || v[3] != 0 {
		t.Fatalf("valor = % x", v)
	}
	// FILETIME de volta para time.Time.
	ft := binary.LittleEndian.Uint64(v[4:])
	got := time.Unix(0, int64(ft-filetimeUnix)*100).UTC()
	if !got.Equal(time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)) {
		t.Errorf("FILETIME = %v", got)
	}
}

func TestDevDriveAntesDosCaches(t *testing.T) {
	pos := map[string]int{}
	for i, m := range All() {
		pos[m.ID] = i
	}
	for _, id := range []string{"dev_tools", "php_sqlsrv", "rust_tools", "performance"} {
		if pos["dev_drive"] > pos[id] {
			t.Errorf("dev_drive precisa rodar antes de %s", id)
		}
	}
}

func TestScriptsDevDrive(t *testing.T) {
	vhd := `C:\DevDrive\DevDrive.vhdx`
	criar := criarDevDriveScript("D", vhd, 150)
	for _, want := range []string{"maximum=153600 type=expandable", "-DevDrive", "-DriveLetter D", `$vhd = 'C:\DevDrive\DevDrive.vhdx'`} {
		if !strings.Contains(criar, want) {
			t.Errorf("criar sem %q:\n%s", want, criar)
		}
	}
	// O argumento da tarefa é um literal do PowerShell com o caminho entre
	// aspas simples dobradas.
	if tarefa := tarefaDevDriveScript(vhd); !strings.Contains(tarefa, `-ImagePath ''C:\DevDrive\DevDrive.vhdx''`) || !strings.Contains(tarefa, "-AtStartup") {
		t.Errorf("tarefa:\n%s", tarefa)
	}
}

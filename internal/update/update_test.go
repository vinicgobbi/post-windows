package update

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestNewer(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"v1.6.0", "v1.5.0", true},
		{"v1.5.1", "v1.5.0", true},
		{"v2.0.0", "v1.99.99", true},
		{"v1.10.0", "v1.9.0", true},
		{"v1.5.0", "v1.5.0", false},
		{"v1.4.9", "v1.5.0", false},
		{"v1.6.0", "dev", false},
		{"lixo", "v1.5.0", false},
	}
	for _, tt := range tests {
		if got := Newer(tt.a, tt.b); got != tt.want {
			t.Errorf("Newer(%q, %q) = %v, esperado %v", tt.a, tt.b, got, tt.want)
		}
	}
}

type entrada struct{ nome, conteudo string }

func montarPacote(t *testing.T, entradas []entrada) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range entradas {
		w, err := zw.Create(e.nome)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(e.conteudo)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

var pacoteTeste = []entrada{
	{"post-windows/post-windows.exe", "executável novo"},
	{"post-windows/README.md", "readme novo"},
	{"post-windows/OVPN/README.md", "ovpn readme novo"},
	{"post-windows/Wallpapers/novo.jpg", "jpg"},
	// Nada disto pode ser escrito:
	{"post-windows/OVPN/empresa.ovpn", "ovpn falso"},
	{"post-windows/logs/x.log", "log falso"},
	{"post-windows/../fora.txt", "fuga"},
	{"post-windows/C:/Windows/x.dll", "unidade"},
	{"/Windows/System32/x.dll", "absoluto"},
}

func TestExtrair(t *testing.T) {
	dir := t.TempDir()
	arq := filepath.Join(dir, "p.zip")
	if err := os.WriteFile(arq, montarPacote(t, pacoteTeste), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := extrair(arq, filepath.Join(dir, "x"), true)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"post-windows.exe", "README.md", "OVPN/README.md", "Wallpapers/novo.jpg"}
	if len(got) != len(want) {
		t.Errorf("extrair = %v, esperado só %v", got, want)
	}
	for _, w := range want {
		if _, ok := got[w]; !ok {
			t.Errorf("faltou %s", w)
		}
	}

	// Executável solto, fora do pacote: só o executável.
	got, err = extrair(arq, filepath.Join(dir, "y"), false)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got["post-windows.exe"]; len(got) != 1 || !ok {
		t.Errorf("extrair sem pacote completo = %v", got)
	}
}

func TestApply(t *testing.T) {
	zipBytes := montarPacote(t, pacoteTeste)
	soma := sha256.Sum256(zipBytes)
	hash := hex.EncodeToString(soma[:])

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/pacote":
			w.Write(zipBytes)
		case "/sha256":
			w.Write([]byte(hash + "  " + pacote + "\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	antigo := client
	client = srv.Client()
	defer func() { client = antigo }()

	base := t.TempDir()
	for nome, conteudo := range map[string]string{
		"post-windows.exe":     "executável velho",
		"Wallpapers/velho.jpg": "jpg velho",
		"OVPN/empresa.ovpn":    "meu perfil",
		"logs/setup.log":       "meu log",
	} {
		p := filepath.Join(base, nome)
		os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(conteudo), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	exePath := filepath.Join(base, "post-windows.exe")

	rel := &Release{Tag: "v9.9.9", url: srv.URL + "/pacote", sumURL: srv.URL + "/sha256", digest: hash}
	if err := Apply(context.Background(), rel, exePath); err != nil {
		t.Fatal(err)
	}
	for nome, want := range map[string]string{
		"post-windows.exe":     "executável novo",
		"post-windows.exe.old": "executável velho",
		"Wallpapers/novo.jpg":  "jpg",
		"Wallpapers/velho.jpg": "jpg velho",
		"OVPN/empresa.ovpn":    "meu perfil",
		"logs/setup.log":       "meu log",
	} {
		got, err := os.ReadFile(filepath.Join(base, nome))
		if err != nil || string(got) != want {
			t.Errorf("%s = %q (%v), esperado %q", nome, got, err, want)
		}
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(base), "fora.txt")); err == nil {
		t.Error("entrada com ../ escreveu fora da pasta")
	}
	if sobras, _ := filepath.Glob(filepath.Join(base, ".post-windows-update-*")); len(sobras) > 0 {
		t.Errorf("diretório temporário não removido: %v", sobras)
	}

	// A próxima execução apaga o executável antigo.
	LimparAntigo(exePath)
	if _, err := os.Stat(Antigo(exePath)); err == nil {
		t.Error("LimparAntigo não apagou o executável antigo")
	}

	// SHA-256 que não confere: nada é alterado.
	rel.digest = "00" + hash[2:]
	os.WriteFile(exePath, []byte("executável velho"), 0o644)
	if err := Apply(context.Background(), rel, exePath); err == nil {
		t.Fatal("Apply aceitou um pacote com SHA-256 diferente")
	}
	if got, _ := os.ReadFile(exePath); string(got) != "executável velho" {
		t.Errorf("executável alterado mesmo com SHA-256 errado: %q", got)
	}
}

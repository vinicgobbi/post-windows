package runner

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/vinicgobbi/post-windows/internal/runlog"
)

func TestSemDowngrade(t *testing.T) {
	req := func(u string) *http.Request {
		p, _ := url.Parse(u)
		return &http.Request{URL: p}
	}
	via := []*http.Request{req("https://github.com/x")}
	if err := semDowngrade(req("https://objects.githubusercontent.com/y"), via); err != nil {
		t.Errorf("https -> https recusado: %v", err)
	}
	if err := semDowngrade(req("http://exemplo.test/y"), via); err == nil {
		t.Error("https -> http aceito")
	}
}

func TestVerifySHA256(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a")
	os.WriteFile(p, []byte("abc"), 0o644)
	const abc = "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	if err := VerifySHA256(p, abc); err != nil {
		t.Error(err)
	}
	if err := VerifySHA256(p, "00"+abc[2:]); err == nil {
		t.Error("SHA-256 diferente aceito")
	}
}

func TestRemoveLine(t *testing.T) {
	log, _ := runlog.New("", "")
	r := &Runner{Ctx: context.Background(), Log: log}
	p := filepath.Join(t.TempDir(), "perfil.ps1")
	os.WriteFile(p, []byte("fnm env | iex\r\n. \"C:\\x\\profile.ps1\"\r\n  fnm env | iex  \r\n"), 0o644)
	if err := r.RemoveLine(p, "fnm env | iex"); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(p); string(got) != ". \"C:\\x\\profile.ps1\"\r\n" {
		t.Errorf("perfil = %q", got)
	}
	// Arquivo que não existe não é erro.
	if err := r.RemoveLine(filepath.Join(t.TempDir(), "nao"), "x"); err != nil {
		t.Error(err)
	}
}

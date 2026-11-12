package runner

import (
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/vinicgobbi/post-windows/internal/winapi"
	"github.com/vinicgobbi/post-windows/internal/winpath"
)

// Host é por onde o runner consulta a máquina e a rede: tudo o que roda
// mesmo no dry-run (comandos e scripts de consulta, binários no PATH,
// variáveis de ambiente, registro, arquivos e requisições HTTP). Com
// Runner.Host nil, as consultas vão para a máquina real; os testes golden
// injetam um Host falso para que o --dry-run de cada módulo seja
// reproduzível (e igual no Linux e no Windows).
type Host interface {
	// Consultar executa um comando somente-leitura e devolve stdout e
	// stderr; err não nil equivale a código de saída diferente de zero. Os
	// scripts do PowerShell chegam como name "powershell" e o script (sem o
	// prelúdio fixo) como único argumento.
	Consultar(name string, args []string) (stdout, stderr []byte, err error)
	LookPath(name string) (string, error)
	Getenv(name string) string
	// ValueNames lista os nomes dos valores de uma chave do registro.
	ValueNames(key string) ([]string, error)
	// FS é a raiz da máquina: "C:\Users\x" vira "C:/Users/x".
	FS() fs.FS
	Transport() http.RoundTripper
}

// LookPath procura um binário no PATH.
func (r *Runner) LookPath(name string) (string, error) {
	if r.Host != nil {
		return r.Host.LookPath(name)
	}
	return exec.LookPath(name)
}

// Exists informa se um binário está no PATH (veja também a função Exists,
// que sempre consulta a máquina real).
func (r *Runner) Exists(name string) bool {
	_, err := r.LookPath(name)
	return err == nil
}

// Getenv lê uma variável de ambiente deste processo.
func (r *Runner) Getenv(name string) string {
	if r.Host != nil {
		return r.Host.Getenv(name)
	}
	return os.Getenv(name)
}

// ValueNames lista os nomes dos valores de uma chave do registro.
func (r *Runner) ValueNames(key string) ([]string, error) {
	if r.Host != nil {
		return r.Host.ValueNames(key)
	}
	return winapi.ValueNames(key)
}

// ReadFile lê um arquivo da máquina.
func (r *Runner) ReadFile(p string) ([]byte, error) {
	if r.Host != nil {
		return fs.ReadFile(r.Host.FS(), caminhoFS(p))
	}
	return os.ReadFile(p)
}

// Stat devolve as informações de um arquivo da máquina.
func (r *Runner) Stat(p string) (fs.FileInfo, error) {
	if r.Host != nil {
		return fs.Stat(r.Host.FS(), caminhoFS(p))
	}
	return os.Stat(p)
}

// FileExists informa se o caminho existe.
func (r *Runner) FileExists(p string) bool {
	_, err := r.Stat(p)
	return err == nil
}

// ReadDir lista uma pasta da máquina.
func (r *Runner) ReadDir(p string) ([]fs.DirEntry, error) {
	if r.Host != nil {
		return fs.ReadDir(r.Host.FS(), caminhoFS(p))
	}
	return os.ReadDir(p)
}

// Glob equivale a filepath.Glob.
func (r *Runner) Glob(pattern string) ([]string, error) {
	if r.Host == nil {
		return filepath.Glob(pattern)
	}
	achados, err := fs.Glob(r.Host.FS(), caminhoFS(pattern))
	for i, a := range achados {
		achados[i] = strings.ReplaceAll(a, "/", `\`)
	}
	return achados, err
}

// WalkDir equivale a filepath.WalkDir.
func (r *Runner) WalkDir(root string, fn fs.WalkDirFunc) error {
	if r.Host == nil {
		return filepath.WalkDir(root, fn)
	}
	return fs.WalkDir(r.Host.FS(), caminhoFS(root), func(p string, d fs.DirEntry, err error) error {
		return fn(strings.ReplaceAll(p, "/", `\`), d, err)
	})
}

// EvalSymlinks resolve os links do caminho (a máquina falsa não tem links).
func (r *Runner) EvalSymlinks(p string) (string, error) {
	if r.Host != nil {
		if !r.FileExists(p) {
			return "", fmt.Errorf("%s: %w", p, fs.ErrNotExist)
		}
		return p, nil
	}
	return filepath.EvalSymlinks(p)
}

// consultarHost é o Probe/Output via Host, com o mesmo registro no .raw.log
// da execução real.
func (r *Runner) consultarHost(desc, name string, args []string) ([]byte, error) {
	out, errOut, err := r.Host.Consultar(name, args)
	r.Log.RawWrite(fmt.Sprintf("$ %s\n%s%s", desc, out, errOut))
	if err != nil {
		return out, fmt.Errorf("%s: %w", firstLine(desc), err)
	}
	return out, nil
}

// caminhoFS converte um caminho do Windows ("C:\Users\x") para o formato do
// fs.FS da máquina falsa ("C:/Users/x").
func caminhoFS(p string) string {
	c := path.Clean(strings.ReplaceAll(winpath.Clean(p), `\`, "/"))
	c = strings.TrimPrefix(c, "/")
	if c == "" {
		return "."
	}
	return c
}

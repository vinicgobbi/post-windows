// Package update confere se há uma release mais nova do post-windows no
// GitHub e, se o usuário quiser, substitui o executável (e, quando ele roda
// de dentro do pacote da release, também Wallpapers/ e os READMEs) pela
// versão nova. O processo atual continua com a versão antiga: é preciso
// executar de novo depois de atualizar.
package update

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	repo   = "vinicgobbi/post-windows"
	pacote = "post-windows-windows-amd64.zip"
	exe    = "post-windows.exe"
	// maxArquivo limita cada arquivo extraído (o maior hoje é um papel de
	// parede 4K de poucos MB), para um pacote adulterado não lotar o disco.
	maxArquivo = 256 << 20
)

// Release é uma versão nova disponível.
type Release struct {
	Tag    string
	url    string // .zip da release
	sumURL string // .sha256 do .zip
	digest string // SHA-256 calculado pelo GitHub no upload (pode faltar)
}

// semDowngrade recusa redirecionamentos de https:// para http://.
func semDowngrade(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return errors.New("redirecionamentos demais")
	}
	if req.URL.Scheme != "https" {
		return fmt.Errorf("redirecionamento inseguro para %s", req.URL)
	}
	return nil
}

var client = &http.Client{CheckRedirect: semDowngrade}

func get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "post-windows")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		resp.Body.Close()
		return nil, fmt.Errorf("GET %s: HTTP %d", url, resp.StatusCode)
	}
	return resp, nil
}

// Check devolve a última release se ela for mais nova que atual, ou nil se
// não houver nada novo. Builds locais (versão "dev" ou fora do formato
// vX.Y.Z) não são conferidos.
func Check(ctx context.Context, atual string) (*Release, error) {
	if _, ok := parseVersao(atual); !ok {
		return nil, nil
	}
	resp, err := get(ctx, "https://api.github.com/repos/"+repo+"/releases/latest")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var rel struct {
		Tag    string `json:"tag_name"`
		Assets []struct {
			Name   string `json:"name"`
			URL    string `json:"browser_download_url"`
			Digest string `json:"digest"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&rel); err != nil {
		return nil, err
	}
	if !Newer(rel.Tag, atual) {
		return nil, nil
	}
	r := &Release{Tag: rel.Tag}
	for _, a := range rel.Assets {
		switch a.Name {
		case pacote:
			r.url = a.URL
			r.digest, _ = strings.CutPrefix(a.Digest, "sha256:")
		case pacote + ".sha256":
			r.sumURL = a.URL
		}
	}
	if r.url == "" || r.sumURL == "" {
		return nil, fmt.Errorf("a release %s não tem %s e o .sha256 dele", rel.Tag, pacote)
	}
	return r, nil
}

// parseVersao lê "vX.Y.Z" (o "v" é opcional).
func parseVersao(v string) ([3]int, bool) {
	var out [3]int
	partes := strings.Split(strings.TrimPrefix(v, "v"), ".")
	if len(partes) != 3 {
		return out, false
	}
	for i, p := range partes {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return out, false
		}
		out[i] = n
	}
	return out, true
}

// Newer diz se a versão a é maior que b (ambas no formato vX.Y.Z).
func Newer(a, b string) bool {
	va, ok1 := parseVersao(a)
	vb, ok2 := parseVersao(b)
	if !ok1 || !ok2 {
		return false
	}
	for i := range va {
		if va[i] != vb[i] {
			return va[i] > vb[i]
		}
	}
	return false
}

// Antigo é onde Apply deixa o executável que estava rodando: o Windows não
// deixa sobrescrever nem apagar um .exe em execução, mas deixa renomear.
// LimparAntigo o apaga na próxima execução.
func Antigo(exePath string) string { return exePath + ".old" }

// LimparAntigo apaga o executável deixado por uma atualização anterior.
func LimparAntigo(exePath string) { os.Remove(Antigo(exePath)) }

// Apply baixa a release, confere o SHA-256 e substitui o executável
// exePath. Se ele estiver dentro da estrutura do pacote da release (com
// Wallpapers\ ao lado), atualiza também Wallpapers\, README.md e
// OVPN\README.md — os .ovpn e os logs nunca são tocados.
func Apply(ctx context.Context, r *Release, exePath string) error {
	base := filepath.Dir(exePath)
	completo := isDir(filepath.Join(base, "Wallpapers"))

	// Tudo é preparado num diretório temporário na mesma pasta (mesma
	// unidade, para os renames finais não copiarem nada) e só então movido
	// para o lugar, com o executável por último.
	stage, err := os.MkdirTemp(base, ".post-windows-update-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)

	arq := filepath.Join(stage, pacote)
	if err := baixar(ctx, r.url, arq); err != nil {
		return err
	}
	if err := conferir(ctx, r, arq); err != nil {
		return err
	}
	arquivos, err := extrair(arq, filepath.Join(stage, "pacote"), completo)
	if err != nil {
		return err
	}
	novo, ok := arquivos[exe]
	if !ok {
		return errors.New("o pacote da release não contém o executável")
	}

	for rel, origem := range arquivos {
		if rel == exe {
			continue
		}
		destino := filepath.Join(base, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(destino), 0o755); err != nil {
			return err
		}
		if err := os.Rename(origem, destino); err != nil {
			return err
		}
	}
	return trocarExecutavel(novo, exePath)
}

// trocarExecutavel põe novo no lugar de exePath. O executável atual (em
// uso) é renomeado para Antigo; se o novo não puder entrar, o atual volta.
func trocarExecutavel(novo, exePath string) error {
	antigo := Antigo(exePath)
	os.Remove(antigo)
	if err := os.Rename(exePath, antigo); err != nil {
		return err
	}
	if err := os.Rename(novo, exePath); err != nil {
		if rerr := os.Rename(antigo, exePath); rerr != nil {
			return fmt.Errorf("%w (e o executável antigo ficou em %s: %v)", err, antigo, rerr)
		}
		return err
	}
	return nil
}

func baixar(ctx context.Context, url, dest string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	resp, err := get(ctx, url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	f, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, io.LimitReader(resp.Body, 1<<30)); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// conferir compara o SHA-256 do pacote com o .sha256 publicado na release e
// com o digest que o GitHub calculou no upload.
func conferir(ctx context.Context, r *Release, arq string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	resp, err := get(ctx, r.sumURL)
	if err != nil {
		return err
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	resp.Body.Close()
	if err != nil {
		return err
	}
	campos := strings.Fields(string(body))
	if len(campos) == 0 {
		return errors.New("arquivo .sha256 da release vazio")
	}

	f, err := os.Open(arq)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))
	for _, want := range []string{campos[0], r.digest} {
		if want != "" && !strings.EqualFold(got, want) {
			return fmt.Errorf("SHA-256 do pacote (%s) não confere com o da release (%s)", got, want)
		}
	}
	return nil
}

// permitido diz se um caminho do pacote (relativo a "post-windows/") deve
// ser atualizado.
func permitido(rel string, completo bool) bool {
	if rel == exe {
		return true
	}
	if !completo {
		return false
	}
	return rel == "README.md" || rel == "OVPN/README.md" || strings.HasPrefix(rel, "Wallpapers/")
}

// extrair descompacta os arquivos permitidos do pacote em dir e devolve
// caminho relativo -> arquivo extraído. Só arquivos regulares; caminhos
// absolutos ou com ".." são ignorados.
func extrair(arq, dir string, completo bool) (map[string]string, error) {
	zr, err := zip.OpenReader(arq)
	if err != nil {
		return nil, err
	}
	defer zr.Close()

	out := map[string]string{}
	for _, f := range zr.File {
		if !f.Mode().IsRegular() {
			continue
		}
		nome := strings.ReplaceAll(f.Name, `\`, "/")
		rel, ok := strings.CutPrefix(path.Clean(nome), "post-windows/")
		if !ok || rel == "" || strings.HasPrefix(rel, "../") || path.IsAbs(rel) || strings.Contains(rel, ":") || !permitido(rel, completo) {
			continue
		}
		if f.UncompressedSize64 > maxArquivo {
			return nil, fmt.Errorf("%s no pacote é grande demais (%d bytes)", rel, f.UncompressedSize64)
		}
		dest := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return nil, err
		}
		if err := extrairArquivo(f, dest); err != nil {
			return nil, err
		}
		out[rel] = dest
	}
	return out, nil
}

func extrairArquivo(f *zip.File, dest string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	w, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(w, io.LimitReader(rc, maxArquivo)); err != nil {
		w.Close()
		return err
	}
	return w.Close()
}

func isDir(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

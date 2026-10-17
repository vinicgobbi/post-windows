// Package runner executa comandos externos, scripts PowerShell, downloads,
// escritas de arquivo e de registro em nome dos módulos, mandando toda a
// saída para o log.
//
// Em modo DryRun, tudo que altera o sistema (comandos "Run"/"Try", scripts
// "PowerShell", escrita de arquivos e de registro, downloads) só é registrado
// no log; consultas somente-leitura (Output/Probe/PSOutput/requisições HTTP/
// leitura de registro) continuam sendo executadas de verdade, para o fluxo dos
// módulos seguir o mesmo caminho que seguiria numa execução real.
package runner

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/vinicgobbi/post-windows/internal/runlog"
	"github.com/vinicgobbi/post-windows/internal/winapi"
)

type Runner struct {
	Ctx    context.Context
	Log    *runlog.Logger
	DryRun bool
	// WorkDir é um diretório temporário exclusivo desta execução, removido
	// no final.
	WorkDir string
}

func (r *Runner) command(name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(r.Ctx, name, args...)
	// Stdin fica no NUL: nenhum comando pode roubar as teclas da TUI nem
	// ficar parado esperando resposta.
	cmd.Stdin = nil
	winapi.PrepareCmd(cmd)
	cmd.WaitDelay = 5 * time.Second
	return cmd
}

// describe monta a linha "$ comando args" mostrada no log. Scripts de várias
// linhas viram uma linha só, separada por "; ".
func describe(name string, args []string) string {
	parts := []string{name}
	for _, a := range args {
		if strings.Contains(a, "\n") {
			a = "'" + umaLinha(a) + "'"
		}
		parts = append(parts, a)
	}
	return strings.Join(parts, " ")
}

func umaLinha(script string) string {
	var linhas []string
	for _, l := range strings.Split(script, "\n") {
		if l = strings.TrimSpace(l); l != "" && !strings.HasPrefix(l, "#") {
			linhas = append(linhas, l)
		}
	}
	return strings.Join(linhas, "; ")
}

// Run executa um comando que altera o sistema, transmitindo a saída para o
// log. Falha se o comando sair com código diferente de zero.
func (r *Runner) Run(name string, args ...string) error {
	return r.run(describe(name, args), name, args...)
}

func (r *Runner) run(desc, name string, args ...string) error {
	if r.DryRun {
		r.Log.Infof("[dry-run] %s", desc)
		return nil
	}
	w := r.Log.OutputWriter()
	defer w.Close()
	fmt.Fprintf(w, "$ %s\n", desc)

	cmd := r.command(name, args...)
	cmd.Stdout, cmd.Stderr = w, w
	if err := cmd.Run(); err != nil {
		if r.Ctx.Err() != nil {
			return r.Ctx.Err()
		}
		return fmt.Errorf("%s: %w", firstLine(desc), err)
	}
	return nil
}

// ExitCode executa um comando que altera o sistema, como Run, mas devolve o
// código de saída em vez de tratá-lo como erro (o winget usa códigos
// próprios para "já instalado", "reinicie para concluir" etc.). err só vem
// preenchido quando o comando nem chegou a rodar, ou foi cancelado.
func (r *Runner) ExitCode(name string, args ...string) (int, error) {
	err := r.Run(name, args...)
	var exitErr *exec.ExitError
	switch {
	case err == nil:
		return 0, nil
	case errors.As(err, &exitErr):
		return exitErr.ExitCode(), nil
	default:
		return -1, err
	}
}

// Try executa um comando que altera o sistema sem mostrar a saída na tela
// (só no .raw.log) e devolve se deu certo. Equivale a "cmd *> $null".
func (r *Runner) Try(name string, args ...string) bool {
	if r.DryRun {
		r.Log.Infof("[dry-run] %s", describe(name, args))
		return true
	}
	return r.quiet(name, args...) == nil
}

// Probe executa uma consulta somente-leitura (ex.: "sc.exe query X") e
// devolve se ela teve sucesso. Roda mesmo em DryRun.
func (r *Runner) Probe(name string, args ...string) bool {
	return r.quiet(name, args...) == nil
}

func (r *Runner) quiet(name string, args ...string) error {
	cmd := r.command(name, args...)
	var buf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &buf, &buf
	err := cmd.Run()
	r.Log.RawWrite(fmt.Sprintf("$ %s\n%s", describe(name, args), buf.String()))
	return err
}

// Output executa uma consulta somente-leitura e devolve o stdout. Roda mesmo
// em DryRun.
func (r *Runner) Output(name string, args ...string) (string, error) {
	return r.output(describe(name, args), name, args...)
}

func (r *Runner) output(desc, name string, args ...string) (string, error) {
	cmd := r.command(name, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	r.Log.RawWrite(fmt.Sprintf("$ %s\n%s%s", desc, out, stderr.String()))
	if err != nil {
		return string(out), fmt.Errorf("%s: %w", firstLine(desc), err)
	}
	return string(out), nil
}

func firstLine(s string) string {
	if len(s) > 160 {
		return s[:160] + "…"
	}
	return s
}

// Exists informa se um binário está no PATH.
func Exists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// RefreshPath relê o PATH do registro (ver winapi.RefreshPath).
func (r *Runner) RefreshPath() { winapi.RefreshPath() }

// ---- PowerShell ----

// psPrelude vem antes de todo script: para no primeiro erro, sem barras de
// progresso (o Write-Progress desenha direto no console) e saída em UTF-8
// (sem isso os acentos chegam na página de código OEM).
const psPrelude = "$ErrorActionPreference = 'Stop'\n" +
	"$ProgressPreference = 'SilentlyContinue'\n" +
	"[Console]::OutputEncoding = [System.Text.Encoding]::UTF8\n"

// psArgs monta a chamada do Windows PowerShell 5.1 (presente em todo
// Windows). O script vai em -EncodedCommand (UTF-16LE em base64) porque o
// powershell.exe trata aspas em -Command de um jeito próprio, que quebra
// scripts com aspas duplas.
func psArgs(script string) []string {
	u := utf16.Encode([]rune(psPrelude + script))
	b := make([]byte, len(u)*2)
	for i, c := range u {
		b[2*i], b[2*i+1] = byte(c), byte(c>>8)
	}
	return []string{"-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass",
		"-EncodedCommand", base64.StdEncoding.EncodeToString(b)}
}

// PowerShell executa um script que altera o sistema.
func (r *Runner) PowerShell(script string) error {
	return r.run("powershell: "+umaLinha(script), "powershell.exe", psArgs(script)...)
}

// PSOutput executa um script somente-leitura e devolve o stdout (sem espaços
// nas pontas). Roda mesmo em DryRun.
func (r *Runner) PSOutput(script string) (string, error) {
	out, err := r.output("powershell: "+umaLinha(script), "powershell.exe", psArgs(script)...)
	return strings.TrimSpace(out), err
}

// PSQuote devolve s como literal de texto do PowerShell ('...').
func PSQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// Deelevated executa um programa como o usuário atual SEM elevação, mesmo
// com este processo rodando como Administrador. Existe para instaladores
// que falham quando rodam elevados (ex.: Spotify/WhatsApp pelo winget).
//
// Usa uma tarefa agendada temporária com RunLevel "Limited" — a forma
// suportada de "desalevar" a partir de um processo já elevado sem pedir
// credenciais de novo. A saída do programa vai para um arquivo, que depois
// é copiado para o log. Devolve o código de saída do programa.
func (r *Runner) Deelevated(exe string, args ...string) (int, error) {
	desc := "(sem elevação) " + describe(exe, args)
	if r.DryRun {
		r.Log.Infof("[dry-run] %s", desc)
		return 0, nil
	}
	logFile := r.Temp(fmt.Sprintf("deelevated-%d.log", time.Now().UnixNano()))
	// cmd /c ""exe" args > "log" 2>&1": com mais de duas aspas, o cmd tira
	// só a primeira e a última.
	linha := `/c ""` + exe + `" ` + winapi.JoinArgs(args) + ` > "` + logFile + `" 2>&1"`
	script := fmt.Sprintf(`
$nome = 'PostWindows_' + [guid]::NewGuid().ToString('N')
$acao = New-ScheduledTaskAction -Execute 'cmd.exe' -Argument %s
$quem = New-ScheduledTaskPrincipal -UserId ("$env:USERDOMAIN\$env:USERNAME") -LogonType Interactive -RunLevel Limited
$cfg = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -ExecutionTimeLimit (New-TimeSpan -Minutes 30)
Register-ScheduledTask -TaskName $nome -Action $acao -Principal $quem -Settings $cfg -Force | Out-Null
try {
    Start-ScheduledTask -TaskName $nome
    Start-Sleep -Seconds 1
    while ((Get-ScheduledTask -TaskName $nome).State -eq 'Running') { Start-Sleep -Milliseconds 500 }
    (Get-ScheduledTaskInfo -TaskName $nome).LastTaskResult
} finally {
    Unregister-ScheduledTask -TaskName $nome -Confirm:$false -ErrorAction SilentlyContinue
}`, PSQuote(linha))

	w := r.Log.OutputWriter()
	fmt.Fprintf(w, "$ %s\n", desc)
	out, err := r.output(desc, "powershell.exe", psArgs(script)...)
	if saida, rerr := os.ReadFile(logFile); rerr == nil {
		w.Write(saida)
	}
	w.Close()
	if err != nil {
		if r.Ctx.Err() != nil {
			return -1, r.Ctx.Err()
		}
		return -1, err
	}
	linhas := strings.Fields(out)
	if len(linhas) == 0 {
		return -1, fmt.Errorf("%s: código de saída não informado", desc)
	}
	code, err := strconv.ParseInt(linhas[len(linhas)-1], 10, 64)
	if err != nil {
		return -1, fmt.Errorf("%s: código de saída inválido %q", desc, linhas[len(linhas)-1])
	}
	// LastTaskResult vem como inteiro sem sinal (ex.: 2316632107); converte
	// para o mesmo int32 com sinal que o exec devolveria.
	return int(int32(uint32(code))), nil
}

// ---- Registro ----

func (r *Runner) SetDWord(key, name string, v uint32) error {
	if r.DryRun {
		r.Log.Infof("[dry-run] registro %s\\%s = %d", key, name, v)
		return nil
	}
	r.Log.RawWrite(fmt.Sprintf("registro %s\\%s = %d\n", key, name, v))
	if err := winapi.SetDWord(key, name, v); err != nil {
		return fmt.Errorf("registro %s\\%s: %w", key, name, err)
	}
	return nil
}

func (r *Runner) SetString(key, name, v string) error {
	if r.DryRun {
		r.Log.Infof("[dry-run] registro %s\\%s = %q", key, name, v)
		return nil
	}
	r.Log.RawWrite(fmt.Sprintf("registro %s\\%s = %q\n", key, name, v))
	if err := winapi.SetString(key, name, v); err != nil {
		return fmt.Errorf("registro %s\\%s: %w", key, name, err)
	}
	return nil
}

func (r *Runner) SetBinary(key, name string, v []byte) error {
	if r.DryRun {
		r.Log.Infof("[dry-run] registro %s\\%s = % x", key, name, v)
		return nil
	}
	r.Log.RawWrite(fmt.Sprintf("registro %s\\%s = % x\n", key, name, v))
	if err := winapi.SetBinary(key, name, v); err != nil {
		return fmt.Errorf("registro %s\\%s: %w", key, name, err)
	}
	return nil
}

// SetUserEnv grava uma variável de ambiente do usuário (HKCU\Environment),
// também neste processo (para os módulos seguintes e os instaladores que
// eles abrirem) e avisa as janelas abertas da mudança.
func (r *Runner) SetUserEnv(name, v string) error {
	if err := r.SetString(`HKCU\Environment`, name, v); err != nil {
		return err
	}
	if !r.DryRun {
		os.Setenv(name, v)
		winapi.BroadcastSettingChange("Environment")
	}
	return nil
}

// SetDWords grava vários valores DWORD na mesma chave, parando no primeiro
// erro.
func (r *Runner) SetDWords(key string, valores map[string]uint32) error {
	for _, nome := range sortedKeys(valores) {
		if err := r.SetDWord(key, nome, valores[nome]); err != nil {
			return err
		}
	}
	return nil
}

func (r *Runner) DeleteValue(key, name string) error {
	if r.DryRun {
		r.Log.Infof("[dry-run] apagar registro %s\\%s", key, name)
		return nil
	}
	r.Log.RawWrite(fmt.Sprintf("apagar registro %s\\%s\n", key, name))
	return winapi.DeleteValue(key, name)
}

func (r *Runner) AddToMachinePath(dir string) error {
	if r.DryRun {
		r.Log.Infof("[dry-run] acrescentar %s ao Path da máquina", dir)
		return nil
	}
	return winapi.AddToMachinePath(dir)
}

// ---- Arquivos ----

func (r *Runner) WriteFile(path, content string) error {
	if r.DryRun {
		r.Log.Infof("[dry-run] escrever %s (%d bytes)", path, len(content))
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

// AppendLine acrescenta uma linha ao arquivo só se ela ainda não estiver lá
// (idempotente em reexecuções). Cria o arquivo e a pasta se não existirem.
func (r *Runner) AppendLine(path, line string) error {
	atual, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if strings.Contains(string(atual), line) {
		return nil
	}
	if r.DryRun {
		r.Log.Infof("[dry-run] acrescentar a %s: %s", path, line)
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if len(atual) > 0 && !bytes.HasSuffix(atual, []byte("\n")) {
		line = "\r\n" + line
	}
	_, err = f.WriteString(line + "\r\n")
	return err
}

// CopyFile copia src para dst (sobrescrevendo).
func (r *Runner) CopyFile(src, dst string) error {
	if r.DryRun {
		r.Log.Infof("[dry-run] copiar %s -> %s", src, dst)
		return nil
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

func (r *Runner) MkdirAll(path string) error {
	if r.DryRun {
		r.Log.Infof("[dry-run] criar pasta %s", path)
		return nil
	}
	return os.MkdirAll(path, 0o755)
}

func (r *Runner) RemoveAll(path string) error {
	if r.DryRun {
		r.Log.Infof("[dry-run] apagar %s", path)
		return nil
	}
	return os.RemoveAll(path)
}

// ---- Rede ----

var httpClient = &http.Client{Timeout: 10 * time.Minute}

// Get faz uma requisição GET somente-leitura com timeout e devolve o corpo.
// Falha em status HTTP de erro.
func (r *Runner) Get(url string, timeout time.Duration) ([]byte, error) {
	ctx, cancel := context.WithTimeout(r.Ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "post-windows")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("GET %s: HTTP %d", url, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// Download baixa uma URL para dest com algumas tentativas antes de desistir,
// para tolerar instabilidade de rede.
func (r *Runner) Download(url, dest string) error {
	if r.DryRun {
		r.Log.Infof("[dry-run] baixar %s -> %s", url, dest)
		return nil
	}
	const tentativas = 3
	var err error
	for i := 1; i <= tentativas; i++ {
		if err = r.downloadOnce(url, dest); err == nil {
			return nil
		}
		if r.Ctx.Err() != nil {
			return r.Ctx.Err()
		}
		r.Log.Warnf("Falha ao baixar %s (tentativa %d/%d): %v", url, i, tentativas, err)
		time.Sleep(time.Duration(i) * time.Second)
	}
	return fmt.Errorf("não foi possível baixar %s após %d tentativas: %w", url, tentativas, err)
}

func (r *Runner) downloadOnce(url, dest string) error {
	req, err := http.NewRequestWithContext(r.Ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "post-windows")
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	tmp := dest + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	r.Log.RawWrite(fmt.Sprintf("download %s -> %s\n", url, dest))
	return os.Rename(tmp, dest)
}

// Temp devolve um caminho dentro do WorkDir.
func (r *Runner) Temp(name string) string {
	return filepath.Join(r.WorkDir, name)
}

func sortedKeys(m map[string]uint32) []string { return slices.Sorted(maps.Keys(m)) }

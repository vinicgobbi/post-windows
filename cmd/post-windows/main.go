// post-windows configura uma workstation Windows recém-instalada: Windows
// Update, Dev Drive, ferramentas de terminal, PowerShell 7 + dotfiles,
// navegadores e apps, Git, VSCode, Go, mise (Node, pnpm, Rust), PHP + SQL
// Server, Ruby, Claude Code, Tailscale, OpenVPN, VirtualBox, papel de
// parede, customização e desempenho — com uma TUI para escolher o
// que rodar.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"

	"github.com/vinicgobbi/post-windows/internal/engine"
	"github.com/vinicgobbi/post-windows/internal/modules"
	"github.com/vinicgobbi/post-windows/internal/plain"
	"github.com/vinicgobbi/post-windows/internal/runlog"
	"github.com/vinicgobbi/post-windows/internal/runner"
	"github.com/vinicgobbi/post-windows/internal/sysinfo"
	"github.com/vinicgobbi/post-windows/internal/tui"
	"github.com/vinicgobbi/post-windows/internal/update"
	"github.com/vinicgobbi/post-windows/internal/winapi"
)

var stdin = bufio.NewReader(os.Stdin)

// version é definido na compilação (-ldflags "-X main.version=...").
var version = "dev"

// flagElevado é acrescentada pelo próprio programa ao se reabrir via UAC:
// a janela nova fecha sozinha ao terminar, então ela espera um Enter.
const flagElevado = "elevated"

type options struct {
	modules  string
	skip     string
	yes      bool
	noTUI    bool
	dryRun   bool
	failFast bool
	list     bool
	version  bool
	elevated bool
	ovpnDir  string
	logDir   string
	wallDir  string
	wall     string
	ddLetter string
	ddGB     int
	noUpdate bool
	// allowUntested aceita rodar numa versão do Windows mais nova que as
	// testadas, sem perguntar.
	allowUntested bool
}

func parseFlags() options {
	var o options
	flag.StringVar(&o.modules, "modules", "", "IDs dos módulos a selecionar, separados por vírgula (padrão: todos)")
	flag.StringVar(&o.skip, "skip", "", "IDs dos módulos a desmarcar, separados por vírgula")
	flag.BoolVar(&o.yes, "yes", false, "não pedir confirmação antes de executar (modo texto)")
	flag.BoolVar(&o.noTUI, "no-tui", false, "usar saída em texto simples em vez da interface interativa")
	flag.BoolVar(&o.dryRun, "dry-run", false, "só mostrar o que seria feito, sem alterar o sistema")
	flag.BoolVar(&o.failFast, "fail-fast", false, "parar tudo no primeiro módulo que falhar")
	flag.BoolVar(&o.list, "list", false, "listar os módulos disponíveis e sair")
	flag.BoolVar(&o.version, "version", false, "mostrar a versão e sair")
	flag.BoolVar(&o.elevated, flagElevado, false, "uso interno: execução reaberta pelo UAC")
	flag.StringVar(&o.ovpnDir, "ovpn-dir", "", "pasta com os arquivos .ovpn a importar (padrão: OVPN\\ ao lado do executável)")
	flag.StringVar(&o.wallDir, "wallpapers-dir", "", "pasta com as imagens de papel de parede (padrão: Wallpapers\\ ao lado do executável)")
	flag.StringVar(&o.wall, "wallpaper", "", "papel de parede a aplicar: nome do arquivo em --wallpapers-dir, ou \"none\" para não alterar (padrão: perguntar)")
	flag.StringVar(&o.logDir, "log-dir", "", "pasta onde os logs da execução são gravados (padrão: logs\\ ao lado do executável)")
	flag.StringVar(&o.ddLetter, "devdrive-letter", "D", "letra da unidade do Dev Drive")
	flag.IntVar(&o.ddGB, "devdrive-size", 150, "tamanho máximo do Dev Drive em GB (VHDX dinâmico: só ocupa o que for usado; mínimo 50)")
	flag.BoolVar(&o.noUpdate, "no-update-check", false, "não verificar se há uma versão mais nova do post-windows")
	flag.BoolVar(&o.allowUntested, "allow-untested-version", false, "aceitar, sem perguntar, rodar numa versão do Windows mais nova que as testadas (por sua conta e risco)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Uso: %s [opções]\n\nOpções:\n", filepath.Base(os.Args[0]))
		flag.VisitAll(func(f *flag.Flag) {
			if f.Name != flagElevado {
				fmt.Fprintf(os.Stderr, "  -%s\n    \t%s\n", f.Name, f.Usage)
			}
		})
	}
	flag.Parse()

	// Por padrão, OVPN\, Wallpapers\ e logs\ ficam ao lado do executável
	// (estrutura do pacote .zip da release), para funcionar de qualquer
	// diretório.
	base := exeDir()
	if o.ovpnDir == "" {
		o.ovpnDir = filepath.Join(base, "OVPN")
	}
	if o.logDir == "" {
		o.logDir = filepath.Join(base, "logs")
	}
	if o.wallDir == "" {
		o.wallDir = filepath.Join(base, "Wallpapers")
	}
	// Absolutos: a janela reaberta pelo UAC começa em outro diretório.
	for _, d := range []*string{&o.ovpnDir, &o.logDir, &o.wallDir} {
		if abs, err := filepath.Abs(*d); err == nil {
			*d = abs
		}
	}
	return o
}

// resolveWallpaper converte o valor de --wallpaper no caminho da imagem.
// Devolve ok=false quando a flag não foi usada (é preciso perguntar).
func resolveWallpaper(walls []string, name string) (path string, ok bool, err error) {
	switch name {
	case "":
		return "", false, nil
	case "none":
		return "", true, nil
	}
	for _, w := range walls {
		if strings.EqualFold(filepath.Base(w), name) {
			return w, true, nil
		}
	}
	return "", false, fmt.Errorf("papel de parede %q não encontrado na pasta de papéis de parede", name)
}

// exePath devolve o caminho real do executável (sem links simbólicos).
func exePath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(exe)
}

// exeDir devolve a pasta do executável (ou o diretório atual, se não der
// para descobrir).
func exeDir() string {
	if exe, err := exePath(); err == nil {
		return filepath.Dir(exe)
	}
	return "."
}

func main() {
	o := parseFlags()
	code := run(o)
	if o.elevated {
		fmt.Print("\nPressione Enter para fechar esta janela...")
		stdin.ReadString('\n')
	}
	os.Exit(code)
}

func run(o options) int {
	all := modules.All()

	if o.version {
		fmt.Println("post-windows", version)
		return 0
	}
	if o.list {
		listar(all)
		return 0
	}

	out := &plain.Printer{W: os.Stdout, Color: term.IsTerminal(int(os.Stdout.Fd()))}
	fail := func(format string, a ...any) int {
		out.Handle(runlog.Entry{Level: runlog.Error, Text: fmt.Sprintf(format, a...)})
		return 1
	}

	sel, avisos, err := initialSelection(all, o.modules, o.skip)
	if err != nil {
		return fail("%v", err)
	}
	if err := validarDevDrive(&o); err != nil {
		return fail("%v", err)
	}

	// Elevação: precisa ser Administrador (o equivalente ao sudo). Reabre o
	// próprio executável pelo UAC, com as mesmas flags, numa janela nova.
	if !o.dryRun && !winapi.IsAdmin() {
		if o.elevated {
			return fail("A execução elevada continua sem direitos de Administrador.")
		}
		out.Warn("É preciso rodar como Administrador. Reabrindo com o UAC...")
		if err := winapi.Elevate(append(os.Args[1:], "--"+flagElevado)); err != nil {
			return fail("Elevação cancelada ou recusada: %v", err)
		}
		return 0
	}
	for _, a := range avisos {
		out.Warn("%s", a)
	}

	// O executável substituído por uma atualização anterior (não dá para
	// apagar um .exe enquanto ele roda).
	if exe, err := exePath(); err == nil {
		update.LimparAntigo(exe)
	}

	if !runner.Exists("winget") {
		return fail("winget não encontrado. Atualize o \"Instalador de Aplicativo\" (App Installer) pela Microsoft Store e rode de novo.")
	}

	sys, err := sysinfo.Detect()
	if err != nil {
		return fail("%v", err)
	}

	log, err := runlog.New(o.logDir, "setup")
	if err != nil {
		return fail("%v", err)
	}
	defer log.Close()

	workDir, err := os.MkdirTemp("", "post-windows-")
	if err != nil {
		return fail("%v", err)
	}
	defer os.RemoveAll(workDir)

	p := preparo{walls: modules.ListWallpapers(o.wallDir)}
	if !o.noUpdate {
		p.upd = verificarAtualizacao(out)
	}
	p.esc.Wallpaper, p.wallChosen, err = resolveWallpaper(p.walls, o.wall)
	if err != nil {
		return fail("%v", err)
	}

	user := os.Getenv("USERNAME")
	start := func(ctx context.Context, esc modules.Escolhas, mods []modules.Module, emit func(any)) []engine.Result {
		log.SetSink(func(e runlog.Entry) { emit(e) })
		log.Infof("Sistema detectado: %s", sys.Summary())
		if sys.NaoTestada {
			log.Warnf("Versão não testada (testadas: Windows 11 %s): execução aceita por conta e risco do usuário.", strings.Join(sys.Testadas, ", "))
		}
		log.Infof("Configuração será aplicada para: %s", user)
		if log.CleanPath != "" {
			log.Infof("Log desta execução: %s", log.CleanPath)
		}
		c := &modules.Ctx{
			Runner:         &runner.Runner{Ctx: ctx, Log: log, DryRun: o.dryRun, WorkDir: workDir},
			Sys:            sys,
			OVPNDir:        o.ovpnDir,
			DevDriveLetter: o.ddLetter,
			DevDriveGB:     o.ddGB,
			Wallpaper:      esc.Wallpaper,
		}
		return engine.Run(ctx, mods, c, engine.Options{FailFast: o.failFast}, emit)
	}

	logPaths := []string{log.CleanPath, log.RawPath}
	interactive := !o.noTUI && term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
	if interactive {
		return runTUI(o, sys, all, sel, user, p, logPaths, start)
	}
	return runPlain(o, sys, all, sel, p, out, start)
}

// listar imprime os módulos agrupados, como no checklist da TUI.
func listar(all []modules.Module) {
	for i, m := range all {
		if i == 0 || m.Grupo != all[i-1].Grupo {
			if i > 0 {
				fmt.Println()
			}
			fmt.Println(m.Grupo + ":")
		}
		deps := ""
		if len(m.Deps) > 0 {
			deps = " (requer: " + strings.Join(m.Deps, ", ") + ")"
		}
		fmt.Printf("  %-22s %s%s\n", m.ID, m.Title, deps)
	}
}

// preparo reúne o que é levantado antes da interface e as escolhas que já
// vieram por flag.
type preparo struct {
	esc        modules.Escolhas
	walls      []string
	wallChosen bool
	upd        *update.Release // versão nova disponível (nil = nenhuma)
}

type startFunc func(ctx context.Context, esc modules.Escolhas, mods []modules.Module, emit func(any)) []engine.Result

// verificarAtualizacao consulta a última release (com prazo curto, para não
// atrasar o início sem rede). Falhas só viram aviso.
func verificarAtualizacao(out *plain.Printer) *update.Release {
	if version == "dev" {
		return nil
	}
	out.Info("Verificando se há versão nova do post-windows...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	rel, err := update.Check(ctx, version)
	if err != nil {
		out.Warn("Não foi possível verificar se há versão nova: %v", err)
		return nil
	}
	return rel
}

// comandoReexecucao monta o comando para executar de novo com as mesmas
// opções, depois de uma atualização (sem a flag interna da elevação).
func comandoReexecucao(exe string) string {
	args := []string{exe}
	for _, a := range os.Args[1:] {
		if a != "--"+flagElevado && a != "-"+flagElevado {
			args = append(args, a)
		}
	}
	return winapi.JoinArgs(args)
}

func runTUI(o options, sys *sysinfo.Info, all []modules.Module, sel []bool, user string, p preparo, logPaths []string, start startFunc) int {
	cfg := tui.Config{
		Sys:          sys,
		Version:      version,
		DryRun:       o.dryRun,
		Modules:      all,
		Selected:     sel,
		User:         user,
		Wallpapers:   p.walls,
		Wallpaper:    p.esc.Wallpaper,
		WallChosen:   p.wallChosen,
		LogPaths:     logPaths,
		VersaoAceita: o.allowUntested,
		Start: func(ctx context.Context, esc modules.Escolhas, mods []modules.Module, emit func(any)) {
			start(ctx, esc, mods, emit)
		},
	}
	var rerun string
	if exe, err := exePath(); err == nil && p.upd != nil {
		rerun = comandoReexecucao(exe)
		cfg.UpdateVersion = p.upd.Tag
		cfg.RerunCommand = rerun
		cfg.ApplyUpdate = func() error { return update.Apply(context.Background(), p.upd, exe) }
	}
	final, err := tea.NewProgram(tui.New(cfg), tea.WithAltScreen()).Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, "erro na interface:", err)
		return 1
	}
	if final.(tui.Model).Updated() {
		fmt.Printf("post-windows atualizado para %s. Execute de novo para usar a versão nova:\n  %s\n", p.upd.Tag, rerun)
		return 0
	}
	res := final.(tui.Model).Result()
	if res == nil {
		fmt.Println("Nenhum módulo executado.")
		return 0
	}
	// A TUI roda em tela alternativa; reimprime o resumo para ficar no
	// histórico do terminal.
	fmt.Println(tui.Summary(res.Results, res.Elapsed, logPaths))
	return exitCode(res.Results)
}

func runPlain(o options, sys *sysinfo.Info, all []modules.Module, sel []bool, p preparo, out *plain.Printer, start startFunc) int {
	stdinTTY := term.IsTerminal(int(os.Stdin.Fd()))
	out.Info("Sistema detectado: %s", sys.Summary())
	for _, w := range sys.Warnings {
		out.Warn("%s", w)
	}

	if p.upd != nil {
		if code, sair := oferecerAtualizacao(o, p.upd, out, stdinTTY); sair {
			return code
		}
	}

	if code, sair := confirmarVersaoNaoTestada(o, sys, out, stdinTTY); sair {
		return code
	}

	esc := p.esc
	if !p.wallChosen && len(p.walls) > 0 && isSelected(all, sel, "papel_parede") {
		if stdinTTY {
			esc.Wallpaper = promptWallpaper(out, p.walls)
		} else {
			out.Warn("Sem terminal interativo e sem --wallpaper: o papel de parede não será alterado.")
		}
	}

	if o.modules == "" && !stdinTTY {
		out.Warn("Entrada não é um terminal interativo: executando todos os módulos (exceto os de --skip).")
	}
	for _, msg := range engine.ResolveDeps(all, sel) {
		out.Warn("%s", msg)
	}

	var mods []modules.Module
	for i, m := range all {
		if sel[i] {
			mods = append(mods, m)
		}
	}
	if len(mods) == 0 {
		out.Warn("Nenhum módulo selecionado. Nada a fazer.")
		return 0
	}

	out.Info("Módulos selecionados:")
	for _, m := range mods {
		fmt.Fprintln(out.W, "  - "+m.Title)
	}
	if stdinTTY && !o.yes && !confirm(fmt.Sprintf("Iniciar a configuração com os %d módulos acima?", len(mods))) {
		out.Warn("Operação cancelada pelo usuário.")
		return 0
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	var elapsed engine.FinishedEvent
	results := start(ctx, esc, mods, func(ev any) {
		if f, ok := ev.(engine.FinishedEvent); ok {
			elapsed = f
		}
		out.Handle(ev)
	})
	fmt.Println()
	fmt.Println(tui.Summary(results, elapsed.Elapsed, nil))
	return exitCode(results)
}

// confirmarVersaoNaoTestada pede (modo texto) a confirmação para seguir numa
// versão do Windows mais nova que as testadas. O aviso em si já saiu com os
// demais de sys.Warnings. Sem terminal, só segue com --yes ou
// --allow-untested-version; no dry-run, que não altera nada, não pergunta.
func confirmarVersaoNaoTestada(o options, sys *sysinfo.Info, out *plain.Printer, stdinTTY bool) (code int, sair bool) {
	switch {
	case !sys.NaoTestada || o.dryRun || o.allowUntested:
		return 0, false
	case stdinTTY:
		if confirm("Continuar mesmo assim, aceitando os riscos?") {
			return 0, false
		}
		out.Warn("Operação cancelada pelo usuário.")
		return 0, true
	case o.yes:
		out.Warn("--yes sem terminal interativo: seguindo na versão não testada, por sua conta e risco.")
		return 0, false
	}
	out.Handle(runlog.Entry{Level: runlog.Error, Text: "Sem terminal interativo: para seguir numa versão não testada, use --allow-untested-version (ou --yes)."})
	return 1, true
}

// oferecerAtualizacao pergunta (modo texto) se atualiza para a versão nova.
// Devolve sair=true quando a execução deve parar (atualizou ou falhou).
func oferecerAtualizacao(o options, rel *update.Release, out *plain.Printer, stdinTTY bool) (code int, sair bool) {
	novidade := fmt.Sprintf("Nova versão disponível: %s (você está na %s).", rel.Tag, version)
	switch {
	case o.dryRun:
		out.Info("%s Execute sem --dry-run para atualizar.", novidade)
		return 0, false
	case !stdinTTY || o.yes:
		out.Warn("%s Execute de forma interativa para atualizar.", novidade)
		return 0, false
	case !confirmDefaultYes(novidade + " Atualizar agora (depois é preciso executar de novo)?"):
		return 0, false
	}
	exe, err := exePath()
	if err != nil {
		out.Handle(runlog.Entry{Level: runlog.Error, Text: fmt.Sprintf("Falha ao localizar o executável: %v", err)})
		return 1, true
	}
	out.Info("Baixando e conferindo %s...", rel.Tag)
	if err := update.Apply(context.Background(), rel, exe); err != nil {
		out.Handle(runlog.Entry{Level: runlog.Error, Text: fmt.Sprintf("Falha ao atualizar: %v", err)})
		return 1, true
	}
	out.Handle(runlog.Entry{Level: runlog.Success, Text: fmt.Sprintf("post-windows atualizado para %s. Execute de novo para usar a versão nova:", rel.Tag)})
	fmt.Fprintln(out.W, "  "+comandoReexecucao(exe))
	return 0, true
}

func exitCode(results []engine.Result) int {
	code := 0
	for _, r := range results {
		switch r.Status {
		case engine.Canceled:
			return 130
		case engine.Failed:
			code = 1
		}
	}
	return code
}

// initialSelection marca todos os módulos (ou só os de --modules) e
// desmarca os de --skip. Os IDs de versões anteriores (modules.Renomeados)
// valem pelos módulos atuais, com um aviso.
func initialSelection(all []modules.Module, only, skip string) (sel []bool, avisos []string, err error) {
	ids := make([]string, len(all))
	for i, m := range all {
		ids[i] = m.ID
	}
	parse := func(s string) ([]string, error) {
		var out []string
		for _, id := range strings.Split(s, ",") {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			if modules.Removidos[id] {
				avisos = append(avisos, fmt.Sprintf("O módulo %q foi removido: ignorado.", id))
				continue
			}
			if novos, ok := modules.Renomeados[id]; ok {
				avisos = append(avisos, fmt.Sprintf("O módulo %q mudou de nome: usando %s (veja --list).", id, strings.Join(novos, ", ")))
				out = append(out, novos...)
				continue
			}
			if !slices.Contains(ids, id) {
				return nil, fmt.Errorf("módulo desconhecido: %q (veja --list)", id)
			}
			out = append(out, id)
		}
		return out, nil
	}
	onlyIDs, err := parse(only)
	if err != nil {
		return nil, nil, err
	}
	skipIDs, err := parse(skip)
	if err != nil {
		return nil, nil, err
	}
	sel = make([]bool, len(all))
	for i, id := range ids {
		sel[i] = (len(onlyIDs) == 0 || slices.Contains(onlyIDs, id)) && !slices.Contains(skipIDs, id)
	}
	return sel, avisos, nil
}

// validarDevDrive normaliza a letra ("d", "D:" → "D") e confere o tamanho
// mínimo que o Windows aceita para um Dev Drive.
func validarDevDrive(o *options) error {
	l := strings.ToUpper(strings.TrimSuffix(strings.TrimSpace(o.ddLetter), ":"))
	if len(l) != 1 || l[0] < 'D' || l[0] > 'Z' {
		return fmt.Errorf("--devdrive-letter inválida: %q (use uma letra de D a Z)", o.ddLetter)
	}
	o.ddLetter = l
	if o.ddGB < 50 {
		return fmt.Errorf("--devdrive-size precisa ser de pelo menos 50 GB (veio %d)", o.ddGB)
	}
	return nil
}

func isSelected(all []modules.Module, sel []bool, id string) bool {
	for i, m := range all {
		if m.ID == id {
			return sel[i]
		}
	}
	return false
}

// promptWallpaper pergunta qual papel de parede aplicar (modo texto).
func promptWallpaper(out *plain.Printer, walls []string) string {
	fmt.Fprintln(out.W, "[?] Qual papel de parede deseja aplicar?")
	for i, w := range walls {
		fmt.Fprintf(out.W, "  %d) %s\n", i+1, filepath.Base(w))
	}
	fmt.Fprintln(out.W, "  0) Não alterar o papel de parede")
	for {
		fmt.Fprint(out.W, "[?] Número: ")
		line, err := stdin.ReadString('\n')
		if err != nil {
			return ""
		}
		var n int
		if _, err := fmt.Sscanf(strings.TrimSpace(line), "%d", &n); err == nil && n >= 0 && n <= len(walls) {
			if n == 0 {
				return ""
			}
			return walls[n-1]
		}
		out.Warn("Opção inválida.")
	}
}

// confirmDefaultYes é como confirm, mas Enter vazio conta como "sim".
func confirmDefaultYes(question string) bool {
	fmt.Printf("[?] %s [S/n]: ", question)
	line, _ := stdin.ReadString('\n')
	switch strings.TrimSpace(strings.ToLower(line)) {
	case "", "s", "y", "sim", "yes":
		return true
	}
	return false
}

func confirm(question string) bool {
	fmt.Printf("[?] %s [s/N]: ", question)
	line, _ := stdin.ReadString('\n')
	switch strings.TrimSpace(strings.ToLower(line)) {
	case "s", "y", "sim", "yes":
		return true
	}
	return false
}

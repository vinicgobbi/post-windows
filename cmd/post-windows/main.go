// post-windows configura uma workstation Windows recém-instalada: Windows
// Update, navegador/Git, Microsoft 365, ferramentas de desenvolvimento, PHP +
// SQL Server, apps de comunicação/produtividade/jogos, VirtualBox, Tailscale,
// OpenVPN, Claude Code, Rust, perfil do PowerShell, customização, desempenho
// e debloat — com uma TUI para escolher o que rodar.
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

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"

	"github.com/vinicgobbi/post-windows/internal/config"
	"github.com/vinicgobbi/post-windows/internal/engine"
	"github.com/vinicgobbi/post-windows/internal/modules"
	"github.com/vinicgobbi/post-windows/internal/plain"
	"github.com/vinicgobbi/post-windows/internal/runlog"
	"github.com/vinicgobbi/post-windows/internal/runner"
	"github.com/vinicgobbi/post-windows/internal/sysinfo"
	"github.com/vinicgobbi/post-windows/internal/tui"
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
	office   string
	ddLetter string
	ddGB     int
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
	flag.StringVar(&o.logDir, "log-dir", "", "pasta onde os logs da execução são gravados (padrão: logs\\ ao lado do executável)")
	flag.StringVar(&o.office, "office-product", config.OfficeProductID, "Product ID do Microsoft 365 no Office Deployment Tool (ex.: O365HomePremRetail para Personal/Family)")
	flag.StringVar(&o.ddLetter, "devdrive-letter", "D", "letra da unidade do Dev Drive")
	flag.IntVar(&o.ddGB, "devdrive-size", 150, "tamanho máximo do Dev Drive em GB (VHDX dinâmico: só ocupa o que for usado; mínimo 50)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Uso: %s [opções]\n\nOpções:\n", filepath.Base(os.Args[0]))
		flag.VisitAll(func(f *flag.Flag) {
			if f.Name != flagElevado {
				fmt.Fprintf(os.Stderr, "  -%s\n    \t%s\n", f.Name, f.Usage)
			}
		})
	}
	flag.Parse()

	// Por padrão, OVPN\ e logs\ ficam ao lado do executável (estrutura do
	// pacote .zip da release), para funcionar de qualquer diretório.
	base := exeDir()
	if o.ovpnDir == "" {
		o.ovpnDir = filepath.Join(base, "OVPN")
	}
	if o.logDir == "" {
		o.logDir = filepath.Join(base, "logs")
	}
	// Absolutos: a janela reaberta pelo UAC começa em outro diretório.
	for _, d := range []*string{&o.ovpnDir, &o.logDir} {
		if abs, err := filepath.Abs(*d); err == nil {
			*d = abs
		}
	}
	return o
}

// exeDir devolve a pasta do executável (ou o diretório atual, se não der
// para descobrir).
func exeDir() string {
	exe, err := os.Executable()
	if err == nil {
		if exe, err = filepath.EvalSymlinks(exe); err == nil {
			return filepath.Dir(exe)
		}
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
		for _, m := range all {
			deps := ""
			if len(m.Deps) > 0 {
				deps = " (requer: " + strings.Join(m.Deps, ", ") + ")"
			}
			fmt.Printf("%-22s %s%s\n", m.ID, m.Title, deps)
		}
		return 0
	}

	out := &plain.Printer{W: os.Stdout, Color: term.IsTerminal(int(os.Stdout.Fd()))}
	fail := func(format string, a ...any) int {
		out.Handle(runlog.Entry{Level: runlog.Error, Text: fmt.Sprintf(format, a...)})
		return 1
	}

	sel, err := initialSelection(all, o.modules, o.skip)
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

	user := os.Getenv("USERNAME")
	start := func(ctx context.Context, mods []modules.Module, emit func(any)) []engine.Result {
		log.SetSink(func(e runlog.Entry) { emit(e) })
		log.Infof("Sistema detectado: %s", sys.Summary())
		log.Infof("Configuração será aplicada para: %s", user)
		if log.CleanPath != "" {
			log.Infof("Log desta execução: %s", log.CleanPath)
		}
		c := &modules.Ctx{
			Runner:         &runner.Runner{Ctx: ctx, Log: log, DryRun: o.dryRun, WorkDir: workDir},
			Sys:            sys,
			OVPNDir:        o.ovpnDir,
			OfficeProduct:  o.office,
			DevDriveLetter: o.ddLetter,
			DevDriveGB:     o.ddGB,
		}
		return engine.Run(ctx, mods, c, engine.Options{FailFast: o.failFast}, emit)
	}

	logPaths := []string{log.CleanPath, log.RawPath}
	interactive := !o.noTUI && term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
	if interactive {
		return runTUI(o, sys, all, sel, user, logPaths, start)
	}
	return runPlain(o, sys, all, sel, out, start)
}

type startFunc func(ctx context.Context, mods []modules.Module, emit func(any)) []engine.Result

func runTUI(o options, sys *sysinfo.Info, all []modules.Module, sel []bool, user string, logPaths []string, start startFunc) int {
	m := tui.New(tui.Config{
		Sys:      sys,
		Version:  version,
		DryRun:   o.dryRun,
		Modules:  all,
		Selected: sel,
		User:     user,
		LogPaths: logPaths,
		Start: func(ctx context.Context, mods []modules.Module, emit func(any)) {
			start(ctx, mods, emit)
		},
	})
	final, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, "erro na interface:", err)
		return 1
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

func runPlain(o options, sys *sysinfo.Info, all []modules.Module, sel []bool, out *plain.Printer, start startFunc) int {
	stdinTTY := term.IsTerminal(int(os.Stdin.Fd()))
	out.Info("Sistema detectado: %s", sys.Summary())

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
	results := start(ctx, mods, func(ev any) {
		if f, ok := ev.(engine.FinishedEvent); ok {
			elapsed = f
		}
		out.Handle(ev)
	})
	fmt.Println()
	fmt.Println(tui.Summary(results, elapsed.Elapsed, nil))
	return exitCode(results)
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
// desmarca os de --skip.
func initialSelection(all []modules.Module, only, skip string) ([]bool, error) {
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
			if !slices.Contains(ids, id) {
				return nil, fmt.Errorf("módulo desconhecido: %q (veja --list)", id)
			}
			out = append(out, id)
		}
		return out, nil
	}
	onlyIDs, err := parse(only)
	if err != nil {
		return nil, err
	}
	skipIDs, err := parse(skip)
	if err != nil {
		return nil, err
	}
	sel := make([]bool, len(all))
	for i, id := range ids {
		sel[i] = (len(onlyIDs) == 0 || slices.Contains(onlyIDs, id)) && !slices.Contains(skipIDs, id)
	}
	return sel, nil
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

func confirm(question string) bool {
	fmt.Printf("[?] %s [s/N]: ", question)
	line, _ := stdin.ReadString('\n')
	switch strings.TrimSpace(strings.ToLower(line)) {
	case "s", "y", "sim", "yes":
		return true
	}
	return false
}

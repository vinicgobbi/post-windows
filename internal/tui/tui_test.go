package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vinicgobbi/post-windows/internal/engine"
	"github.com/vinicgobbi/post-windows/internal/modules"
	"github.com/vinicgobbi/post-windows/internal/sysinfo"
)

func novoModelo() Model {
	all := modules.All()
	sel := make([]bool, len(all))
	for i := range sel {
		sel[i] = true
	}
	m := New(Config{
		Sys:      &sysinfo.Info{Edition: "Windows 11 Pro", DisplayVersion: "24H2", Build: 26100, Arch: "amd64"},
		Version:  "dev",
		Modules:  all,
		Selected: sel,
		User:     "fulano",
	})
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return mm.(Model)
}

func tecla(m Model, k string) Model {
	var msg tea.KeyMsg
	switch k {
	case "enter":
		msg = tea.KeyMsg{Type: tea.KeyEnter}
	case "space":
		msg = tea.KeyMsg{Type: tea.KeySpace, Runes: []rune(" ")}
	default:
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
	}
	mm, _ := m.Update(msg)
	return mm.(Model)
}

func TestTelasSelecaoEConfirmacao(t *testing.T) {
	m := novoModelo()
	v := m.View()
	if !strings.Contains(v, "POST-WINDOWS") || !strings.Contains(v, "Windows 11 Pro 24H2") || !strings.Contains(v, "fulano") {
		t.Fatalf("cabeçalho:\n%s", v)
	}
	n := len(modules.All())
	if !strings.Contains(v, fmt.Sprintf("(%d de %d selecionados)", n, n)) {
		t.Errorf("seleção:\n%s", v)
	}

	// Os grupos aparecem como cabeçalhos no checklist.
	for _, g := range []string{modules.GrupoSistema, modules.GrupoTerminal, modules.GrupoDesenvolvimento} {
		if !strings.Contains(v, g) {
			t.Errorf("checklist sem o grupo %q:\n%s", g, v)
		}
	}

	// Desmarcar o mise desmarca o Node e o Rust, que dependem dele.
	for _, mod := range modules.All() {
		if mod.ID == "mise" {
			break
		}
		m = tecla(m, "j")
	}
	m = tecla(m, "space")
	if v := m.View(); !strings.Contains(v, "Também desmarcado (depende deste): Node.js e pnpm (mise), Rust, eza e topgrade") ||
		!strings.Contains(v, fmt.Sprintf("(%d de %d selecionados)", n-3, n)) {
		t.Errorf("cascata:\n%s", v)
	}

	m = tecla(m, "enter")
	if v := m.View(); !strings.Contains(v, fmt.Sprintf("Iniciar a configuração com os %d módulos", n-3)) {
		t.Errorf("confirmação:\n%s", v)
	}
}

func TestTelaWallpaper(t *testing.T) {
	all := modules.All()
	sel := make([]bool, len(all))
	for i := range sel {
		sel[i] = true
	}
	cfg := Config{
		Sys:        &sysinfo.Info{Edition: "Windows 11 Pro", Build: 26200, Arch: "amd64"},
		Modules:    all,
		Selected:   sel,
		Wallpapers: []string{`C:\pw\Wallpapers\a.jpg`, `C:\pw\Wallpapers\b.png`},
	}
	m := New(cfg)
	if m.state != stateWallpaper {
		t.Fatalf("estado inicial = %v, esperado a tela de papel de parede", m.state)
	}
	// A segunda imagem.
	m = tecla(m, "j")
	m = tecla(m, "enter")
	if m.state != stateSelect || m.cfg.Wallpaper != cfg.Wallpapers[1] {
		t.Errorf("escolha: estado %v, papel de parede %q", m.state, m.cfg.Wallpaper)
	}

	// "Não alterar" desmarca o módulo.
	m = New(cfg)
	m = tecla(m, "j")
	m = tecla(m, "j")
	m = tecla(m, "enter")
	if m.cfg.Wallpaper != "" || m.marcado("papel_parede") {
		t.Errorf("não alterar: papel de parede %q, módulo marcado %v", m.cfg.Wallpaper, m.marcado("papel_parede"))
	}

	// Com --wallpaper, sem pergunta.
	cfg.WallChosen = true
	if m := New(cfg); m.state != stateSelect {
		t.Errorf("com --wallpaper: estado %v", m.state)
	}
}

func TestSummary(t *testing.T) {
	res := []engine.Result{
		{Module: modules.Module{Title: "A"}, Status: engine.Done, Duration: time.Second},
		{Module: modules.Module{Title: "B"}, Status: engine.DoneWithWarnings, Warnings: 2},
	}
	s := Summary(res, 3*time.Second, []string{`C:\logs\setup.log`})
	if !strings.Contains(s, "2 ok") || !strings.Contains(s, "2 aviso(s)") || !strings.Contains(s, `C:\logs\setup.log`) {
		t.Errorf("summary:\n%s", s)
	}
}

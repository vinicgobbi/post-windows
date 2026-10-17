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

	// Desmarcar o Git desmarca o perfil do PowerShell, que depende dele.
	for _, mod := range modules.All() {
		if mod.ID == "navegador_git" {
			break
		}
		m = tecla(m, "j")
	}
	m = tecla(m, "space")
	if v := m.View(); !strings.Contains(v, "Também desmarcado (depende deste): Configurar perfil do PowerShell") ||
		!strings.Contains(v, fmt.Sprintf("(%d de %d selecionados)", n-2, n)) {
		t.Errorf("cascata:\n%s", v)
	}

	m = tecla(m, "enter")
	if v := m.View(); !strings.Contains(v, fmt.Sprintf("Iniciar a configuração com os %d módulos", n-2)) {
		t.Errorf("confirmação:\n%s", v)
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

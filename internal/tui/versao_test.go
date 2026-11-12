package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vinicgobbi/post-windows/internal/sysinfo"
)

func TestTelaVersao(t *testing.T) {
	sys := &sysinfo.Info{Edition: "Windows 11 Pro", DisplayVersion: "27H1", Build: 28100, NaoTestada: true,
		Testadas: []string{"25H2 (build 26200)", "26H1 (build 28000)", "26H2 (build 26300)"}}
	base := Config{Sys: sys, Version: "v2.0.0", User: "ana"}

	// O padrão é sair.
	m := New(base)
	if m.state != stateVersao {
		t.Fatalf("estado inicial = %v, esperado a tela de versão", m.state)
	}
	if m.viewVersao() == "" {
		t.Error("tela vazia")
	}
	if _, cmd := teclaTipo(m, tea.KeyEnter); cmd == nil {
		t.Error("enter em 'Sair' não saiu")
	}

	// Continuar segue para a tela que viria sem a pergunta.
	m, _ = teclaTipo(m, tea.KeyDown)
	m, _ = teclaTipo(m, tea.KeyEnter)
	if m.state != stateSelect {
		t.Errorf("depois de continuar: estado %v, esperado o checklist", m.state)
	}

	// Sem pergunta: no dry-run, com --allow-untested-version e em versão testada.
	for nome, cfg := range map[string]Config{
		"dry-run": func() Config { c := base; c.DryRun = true; return c }(),
		"aceita":  func() Config { c := base; c.VersaoAceita = true; return c }(),
		"testada": func() Config { c := base; c.Sys = &sysinfo.Info{}; return c }(),
	} {
		if m := New(cfg); m.state == stateVersao {
			t.Errorf("%s: não deveria perguntar", nome)
		}
	}

	// Com versão nova do post-windows, a oferta de atualização vem antes, e
	// ignorá-la leva à pergunta da versão.
	cfg := base
	cfg.UpdateVersion, cfg.ApplyUpdate = "v2.1.0", func() error { return nil }
	m = New(cfg)
	if m.state != stateAtualizacao {
		t.Fatalf("estado inicial = %v, esperado a tela de atualização", m.state)
	}
	m, _ = teclaTipo(m, tea.KeyDown)
	m, _ = teclaTipo(m, tea.KeyEnter)
	if m.state != stateVersao {
		t.Errorf("depois de ignorar a atualização: estado %v, esperado a tela de versão", m.state)
	}
}

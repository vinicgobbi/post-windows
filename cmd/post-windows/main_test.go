package main

import (
	"slices"
	"strings"
	"testing"

	"github.com/vinicgobbi/post-windows/internal/modules"
)

func TestInitialSelection(t *testing.T) {
	all := modules.All()
	marcados := func(sel []bool) []string {
		var ids []string
		for i, s := range sel {
			if s {
				ids = append(ids, all[i].ID)
			}
		}
		return ids
	}

	sel, avisos, err := initialSelection(all, "", "")
	if err != nil || len(avisos) > 0 || len(marcados(sel)) != len(all) {
		t.Fatalf("padrão: %v, %v, %v", marcados(sel), avisos, err)
	}

	// IDs antigos valem pelos novos, com aviso.
	sel, avisos, err = initialSelection(all, "navegador_git,go", "")
	if err != nil || !slices.Equal(marcados(sel), []string{"chrome", "firefox", "git_gcm", "go"}) {
		t.Errorf("renomeado: %v, %v", marcados(sel), err)
	}
	if len(avisos) != 1 || !strings.Contains(avisos[0], "navegador_git") {
		t.Errorf("avisos = %v", avisos)
	}

	sel, avisos, _ = initialSelection(all, "", "performance,rust_tools")
	if got := marcados(sel); slices.Contains(got, "performance") || slices.Contains(got, "rust") || len(got) != len(all)-2 {
		t.Errorf("skip: %v", got)
	}
	if len(avisos) != 1 || !strings.Contains(avisos[0], "rust_tools") {
		t.Errorf("avisos do skip = %v", avisos)
	}

	// Módulo removido: ignorado, com aviso (e sem erro).
	sel, avisos, err = initialSelection(all, "", "debloat")
	if err != nil || len(marcados(sel)) != len(all) || len(avisos) != 1 || !strings.Contains(avisos[0], "removido") {
		t.Errorf("removido: %v, %v, %v", marcados(sel), avisos, err)
	}

	if _, _, err := initialSelection(all, "nao_existe", ""); err == nil {
		t.Error("ID desconhecido deveria dar erro")
	}
}

func TestResolveWallpaper(t *testing.T) {
	walls := []string{`C:\pw\Wallpapers\a.jpg`, `C:\pw\Wallpapers\b.png`}
	if p, ok, err := resolveWallpaper(walls, ""); p != "" || ok || err != nil {
		t.Errorf("vazio: %q %v %v", p, ok, err)
	}
	if p, ok, err := resolveWallpaper(walls, "none"); p != "" || !ok || err != nil {
		t.Errorf("none: %q %v %v", p, ok, err)
	}
	if _, _, err := resolveWallpaper(walls, "c.jpg"); err == nil {
		t.Error("imagem inexistente deveria dar erro")
	}
}

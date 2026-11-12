package sysinfo

import (
	"strings"
	"testing"
)

func TestFromRegistryWindows11(t *testing.T) {
	info, err := fromRegistry(map[string]string{
		"ProductName": "Windows 10 Pro", "EditionID": "Professional",
		"DisplayVersion": "24H2", "CurrentBuildNumber": "26100", "UBR": "4351",
	}, "amd64")
	if err != nil {
		t.Fatal(err)
	}
	if info.Edition != "Windows 11 Pro" || !info.IsPro() {
		t.Errorf("edição = %q", info.Edition)
	}
	if got := info.Summary(); got != "Windows 11 Pro 24H2 (build 26100.4351, amd64)" {
		t.Errorf("Summary = %q", got)
	}
}

func TestFromRegistryWindows10Home(t *testing.T) {
	info, err := fromRegistry(map[string]string{"ProductName": "Windows 10 Home", "CurrentBuildNumber": "19045"}, "amd64")
	if err != nil {
		t.Fatal(err)
	}
	if info.Edition != "Windows 10 Home" || info.IsPro() {
		t.Errorf("info = %+v", info)
	}
}

func TestFromRegistryAntigo(t *testing.T) {
	if _, err := fromRegistry(map[string]string{"ProductName": "Windows 10 Pro", "CurrentBuildNumber": "18363"}, "amd64"); err == nil {
		t.Error("build 18363 deveria ser recusado")
	}
	if _, err := fromRegistry(map[string]string{}, "amd64"); err == nil {
		t.Error("build vazio deveria ser recusado")
	}
}

func TestConferirSuporte(t *testing.T) {
	suportadas := []versaoWindows{{Versao: "25H2", Build: 26200}, {Versao: "26H1", Build: 28000}, {Versao: "26H2", Build: 26300}}

	testada := &Info{Edition: "Windows 11 Pro", DisplayVersion: "25H2", Build: 26200}
	conferirSuporte(testada, suportadas)
	if testada.NaoTestada || len(testada.Warnings) > 0 || len(testada.Testadas) != 3 {
		t.Errorf("testada: %+v", testada)
	}

	nova := &Info{Edition: "Windows 11 Pro", DisplayVersion: "27H1", Build: 28100}
	conferirSuporte(nova, suportadas)
	if !nova.NaoTestada || len(nova.Warnings) != 1 || !strings.Contains(nova.Warnings[0], "mais novo") {
		t.Errorf("nova: %+v", nova)
	}

	// Build entre os testados, mas que não é nenhum deles (26H1 é 28000 e
	// 26H2 é 26300): fora de suporte, não "mais nova".
	for _, b := range []int{19045, 26100, 27000} {
		antiga := &Info{Edition: "Windows 11 Pro", Build: b}
		conferirSuporte(antiga, suportadas)
		if antiga.NaoTestada || len(antiga.Warnings) != 1 || !strings.Contains(antiga.Warnings[0], "fora do suporte") {
			t.Errorf("build %d: %+v", b, antiga)
		}
	}
}

func TestVersoesOrdenadas(t *testing.T) {
	if len(windowsSuportadas) == 0 {
		t.Fatal("versoes.go sem nenhuma versão")
	}
	vistos := map[int]bool{}
	for _, v := range windowsSuportadas {
		if vistos[v.Build] || v.Build < 22000 {
			t.Errorf("build inválido ou repetido: %+v", v)
		}
		vistos[v.Build] = true
	}
}

package sysinfo

import "testing"

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

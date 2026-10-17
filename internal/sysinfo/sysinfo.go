// Package sysinfo detecta a versão, a edição e a arquitetura do Windows.
package sysinfo

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"

	"github.com/vinicgobbi/post-windows/internal/winapi"
)

// MinBuild é o Windows 10 2004, a primeira versão com winget.
const MinBuild = 19041

const chaveVersao = `HKLM\SOFTWARE\Microsoft\Windows NT\CurrentVersion`

type Info struct {
	Edition        string // ex.: "Windows 11 Pro"
	EditionID      string // ex.: "Professional", "Core"
	DisplayVersion string // ex.: "24H2"
	Build          int
	UBR            int // revisão do build (o ".1234" de 26100.1234)
	Arch           string
}

// IsPro informa se a edição tem Hyper-V/gpedit (Pro, Enterprise, Education).
func (i *Info) IsPro() bool {
	for _, s := range []string{"Pro", "Enterprise", "Education"} {
		if strings.Contains(i.Edition, s) {
			return true
		}
	}
	return false
}

// Summary é a linha "Sistema detectado: ..." mostrada no início.
func (i *Info) Summary() string {
	ver := ""
	if i.DisplayVersion != "" {
		ver = " " + i.DisplayVersion
	}
	build := strconv.Itoa(i.Build)
	if i.UBR > 0 {
		build += "." + strconv.Itoa(i.UBR)
	}
	return fmt.Sprintf("%s%s (build %s, %s)", i.Edition, ver, build, i.Arch)
}

// Detect lê a versão do Windows no registro.
func Detect() (*Info, error) {
	val := map[string]string{}
	for _, nome := range []string{"ProductName", "EditionID", "DisplayVersion", "CurrentBuildNumber"} {
		if v, err := winapi.GetString(chaveVersao, nome); err == nil {
			val[nome] = v
		} else if nome == "CurrentBuildNumber" {
			return nil, fmt.Errorf("lendo a versão do Windows: %w", err)
		}
	}
	if ubr, err := winapi.GetDWord(chaveVersao, "UBR"); err == nil {
		val["UBR"] = strconv.FormatUint(ubr, 10)
	}
	return fromRegistry(val, runtime.GOARCH)
}

func fromRegistry(val map[string]string, arch string) (*Info, error) {
	build, err := strconv.Atoi(val["CurrentBuildNumber"])
	if err != nil {
		return nil, fmt.Errorf("build do Windows inválido: %q", val["CurrentBuildNumber"])
	}
	ubr, _ := strconv.Atoi(val["UBR"])
	info := &Info{
		Edition:        val["ProductName"],
		EditionID:      val["EditionID"],
		DisplayVersion: val["DisplayVersion"],
		Build:          build,
		UBR:            ubr,
		Arch:           arch,
	}
	// O ProductName continua "Windows 10 ..." no Windows 11 (a Microsoft
	// nunca atualizou o valor); o build 22000 é o primeiro do Windows 11.
	if build >= 22000 {
		info.Edition = strings.Replace(info.Edition, "Windows 10", "Windows 11", 1)
	}
	if info.Edition == "" {
		info.Edition = "Windows"
	}
	if build < MinBuild {
		return nil, fmt.Errorf("versão do Windows não suportada (build %d): é preciso o Windows 10 2004+ (build %d) ou o Windows 11, onde o winget está disponível", build, MinBuild)
	}
	return info, nil
}

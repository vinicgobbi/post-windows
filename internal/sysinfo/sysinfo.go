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
	// NaoTestada indica um build mais novo que todos os de versoes.go: pode
	// funcionar, mas pede confirmação antes de seguir.
	NaoTestada bool
	// Testadas são as versões de versoes.go, para as mensagens.
	Testadas []string
	// Warnings são avisos sobre o sistema, mostrados no início.
	Warnings []string
}

// versaoWindows é uma versão do Windows 11 com suporte (veja versoes.go).
type versaoWindows struct {
	Versao string // ex.: "25H2"
	Build  int
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
	conferirSuporte(info, windowsSuportadas)
	return info, nil
}

// conferirSuporte compara o build com as versões testadas (versoes.go). Um
// build mais novo que todos pode funcionar, mas pede confirmação
// (NaoTestada). Um build mais antigo que não esteja na lista (o Windows 10, ou
// um Windows 11 que saiu do suporte) continua funcionando — o winget e os
// módulos existem nele —, mas fica sem as correções de segurança da
// Microsoft: só gera um aviso.
func conferirSuporte(info *Info, suportadas []versaoWindows) {
	maior, achou := 0, false
	for _, v := range suportadas {
		info.Testadas = append(info.Testadas, fmt.Sprintf("%s (build %d)", v.Versao, v.Build))
		achou = achou || v.Build == info.Build
		maior = max(maior, v.Build)
	}
	if achou {
		return
	}
	lista := "Windows 11 " + strings.Join(info.Testadas, ", ")
	if info.Build > maior {
		info.NaoTestada = true
		info.Warnings = append(info.Warnings, fmt.Sprintf(
			"%s é mais novo que as versões testadas (%s): pode funcionar, mas não foi testado.", info.nome(), lista))
		return
	}
	info.Warnings = append(info.Warnings, fmt.Sprintf(
		"%s está fora do suporte da Microsoft (sem atualizações de segurança nas edições Home/Pro). "+
			"O post-windows segue, mas foi testado só com %s: atualize o Windows quando puder.", info.nome(), lista))
}

// nome é a edição com a versão e o build (ex.: "Windows 11 Pro 24H2 (build
// 26100)").
func (i *Info) nome() string {
	ver := ""
	if i.DisplayVersion != "" {
		ver = " " + i.DisplayVersion
	}
	return fmt.Sprintf("%s%s (build %d)", i.Edition, ver, i.Build)
}

// Nome é a edição com a versão e o build, para as telas de aviso.
func (i *Info) Nome() string { return i.nome() }

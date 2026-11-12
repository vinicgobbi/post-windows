package modules

import (
	"os"
	"strings"

	"github.com/vinicgobbi/post-windows/internal/winpath"
)

var bootstrap = Module{
	ID:    "bootstrap",
	Grupo: GrupoSistema,
	Title: "Preparar ambiente",
	Desc:  "Atualiza as fontes do winget e prepara NuGet/PSGallery para os demais módulos",
	Run: func(c *Ctx) error {
		c.Log.Infof("Preparando o ambiente (winget, NuGet, PSGallery)...")
		if err := c.Run("winget", "source", "update"); err != nil {
			return err
		}
		// O Install-Module do PSWindowsUpdate depende do provider NuGet; e
		// confiar na PSGallery evita o prompt "repositório não confiável".
		err := c.PowerShell(`if (-not (Get-PackageProvider -ListAvailable -Name NuGet -ErrorAction SilentlyContinue)) {
    Install-PackageProvider -Name NuGet -MinimumVersion 2.8.5.201 -Force | Out-Null
}
if ((Get-PSRepository -Name PSGallery).InstallationPolicy -ne 'Trusted') {
    Set-PSRepository -Name PSGallery -InstallationPolicy Trusted
}`)
		if err != nil {
			return err
		}
		c.Log.Successf("Ambiente preparado.")
		return nil
	},
}

var atualizacao = Module{
	ID:    "atualizacao",
	Grupo: GrupoSistema,
	Title: "Atualizar o Windows",
	Desc:  "Instala o módulo PSWindowsUpdate e aplica as atualizações pendentes do Windows Update (sem reiniciar; avisa no final)",
	Deps:  []string{"bootstrap"},
	Run: func(c *Ctx) error {
		c.Log.Infof("Buscando atualizações do Windows (pode demorar)...")
		// -IgnoreReboot de propósito: reiniciar sozinho no meio da lista
		// derrubaria os módulos seguintes. O aviso de reinício fica no final.
		instalar := `Install-Module -Name PSWindowsUpdate -Force -Scope AllUsers`
		aplicar := `Install-WindowsUpdate -AcceptAll -IgnoreReboot -Confirm:$false | Out-Null
if (Get-WURebootStatus -Silent) { 'REBOOT' } else { 'OK' }`
		if c.DryRun {
			// Só lista o que seria instalado.
			instalar, aplicar = `'SEMMODULO'; return`, `'DRYRUN'`
		}
		out, err := c.PSOutput(`if (-not (Get-Module -ListAvailable -Name PSWindowsUpdate)) { ` + instalar + ` }
Import-Module PSWindowsUpdate
$updates = Get-WindowsUpdate -AcceptAll -ErrorAction SilentlyContinue
if (-not $updates) { 'NADA'; return }
$updates | ForEach-Object { 'UPD ' + $_.Title }
` + aplicar)
		if err != nil {
			return err
		}
		if strings.Contains(out, "SEMMODULO") {
			c.Log.Infof("[dry-run] Install-Module PSWindowsUpdate (sem o módulo, não dá para listar as atualizações)")
			return nil
		}
		for _, l := range strings.Split(out, "\n") {
			if t, ok := strings.CutPrefix(strings.TrimSpace(l), "UPD "); ok {
				c.Log.Infof("  %s", t)
			}
		}
		switch {
		case strings.Contains(out, "NADA"):
			c.Log.Successf("Nenhuma atualização pendente.")
		case strings.Contains(out, "DRYRUN"):
			c.Log.Infof("[dry-run] Install-WindowsUpdate -AcceptAll -IgnoreReboot")
		case strings.Contains(out, "REBOOT"):
			c.Log.Warnf("Atualizações instaladas: é necessário reiniciar o Windows para concluir (reinicie ao final).")
		default:
			c.Log.Successf("Atualizações do Windows instaladas.")
		}
		return nil
	},
}

var limpeza = Module{
	ID:    "limpeza",
	Grupo: GrupoFinalizacao,
	Title: "Limpeza final",
	Desc:  "Remove arquivos temporários, versões antigas dos componentes do Windows (DISM), o cache do Delivery Optimization e o do Chocolatey (se instalado)",
	Run: func(c *Ctx) error {
		c.Log.Infof("Limpando arquivos temporários...")
		for _, dir := range []string{c.Getenv("TEMP"), winpath.Join(c.Getenv("WINDIR"), "Temp")} {
			if dir != "" {
				limparPasta(c, dir)
			}
		}
		if c.Exists("choco") {
			c.Try("choco", "cache", "remove", "--all")
		}

		// Pacotes de atualização já instalados, guardados para compartilhar
		// com outros PCs.
		if err := c.avisarFalha(c.PowerShell(`Delete-DeliveryOptimizationCache -Force`),
			"Não consegui limpar o cache do Delivery Optimization"); err != nil {
			return err
		}

		// Versões antigas dos componentes que sobram depois do Windows
		// Update. Sem /ResetBase de propósito: as atualizações continuam
		// desinstaláveis.
		c.Log.Infof("Limpando versões antigas dos componentes do Windows (pode levar alguns minutos)...")
		if err := c.avisarFalha(c.Run("Dism.exe", "/Online", "/Cleanup-Image", "/StartComponentCleanup"),
			"O DISM não concluiu a limpeza dos componentes"); err != nil {
			return err
		}
		c.Log.Successf("Limpeza concluída.")
		return nil
	},
}

// limparPasta apaga o conteúdo de dir, menos o WorkDir desta execução
// (apagado no final). Arquivos em uso são ignorados.
func limparPasta(c *Ctx, dir string) {
	entradas, err := c.ReadDir(dir)
	if err != nil {
		return
	}
	if c.DryRun {
		c.Log.Infof("[dry-run] apagar o conteúdo de %s (%d itens)", dir, len(entradas))
		return
	}
	apagados := 0
	for _, e := range entradas {
		p := winpath.Join(dir, e.Name())
		if winpath.Igual(p, c.WorkDir) {
			continue
		}
		if os.RemoveAll(p) == nil {
			apagados++
		}
	}
	c.Log.Infof("%s: %d de %d itens apagados (os demais estão em uso).", dir, apagados, len(entradas))
}

package modules

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vinicgobbi/post-windows/internal/runner"
)

// tarefaDevDrive monta o VHDX no boot: um VHDX anexado pelo Mount-DiskImage
// não volta sozinho depois de reiniciar.
const tarefaDevDrive = "PostWindows-DevDrive"

var devDrive = Module{
	ID:    "dev_drive",
	Title: "Criar Dev Drive",
	Desc:  "Disco virtual (VHDX dinâmico) em ReFS para Projects e os caches de pacotes (npm, Composer, pip, Cargo/rustup), com o Defender em modo de desempenho no lugar das exclusões",
	Run: func(c *Ctx) error {
		// O Dev Drive chegou no Windows 11 22H2, build 22621.2338.
		if c.Sys.Build < 22621 || (c.Sys.Build == 22621 && c.Sys.UBR < 2338) {
			c.Log.Warnf("Dev Drive precisa do Windows 11 22H2 (build 22621.2338) ou mais novo; pulando.")
			return nil
		}
		letra := c.DevDriveLetter
		raiz := letra + `:\`
		vhd := vhdDevDrive()

		estado, err := c.PSOutput(estadoDevDriveScript(letra, vhd))
		if err != nil {
			return err
		}
		switch {
		case estado == "REFS":
			c.Log.Successf("Dev Drive %s já existe.", raiz)
		case strings.HasPrefix(estado, "OCUPADA"):
			c.Log.Warnf("A letra %s já está em uso (%s); use --devdrive-letter para escolher outra. Pulando o Dev Drive.",
				raiz, strings.TrimSpace(strings.TrimPrefix(estado, "OCUPADA")))
			return nil
		case estado == "VHD":
			c.Log.Infof("Remontando o Dev Drive de %s...", vhd)
			if err := c.PowerShell(montarDevDriveScript(letra, vhd)); err != nil {
				return err
			}
		default:
			c.Log.Infof("Criando o Dev Drive %s (VHDX dinâmico de até %d GB em %s)...", raiz, c.DevDriveGB, vhd)
			if err := c.PowerShell(criarDevDriveScript(letra, vhd, c.DevDriveGB)); err != nil {
				return err
			}
		}

		if err := c.PowerShell(tarefaDevDriveScript(vhd)); err != nil {
			return err
		}

		// Recém-criado já nasce confiável; o trust cobre um VHDX trazido de
		// outra máquina. É o "confiável" que liga o modo de desempenho do
		// Defender (verificação assíncrona) e limita os filtros de
		// sistema de arquivos.
		c.Try("fsutil", "devdrv", "trust", letra+":")
		if out, err := c.Output("fsutil", "devdrv", "query", letra+":"); err == nil {
			for _, l := range strings.Split(out, "\n") {
				if l = strings.TrimSpace(l); l != "" {
					c.Log.Infof("  %s", l)
				}
			}
		}

		if err := configurarPastasDevDrive(c, raiz); err != nil {
			return err
		}
		c.projetos = filepath.Join(raiz, "Projects")
		c.Log.Successf("Dev Drive %s pronto: Projects e caches de pacotes ficam nele.", raiz)
		return nil
	},
}

func vhdDevDrive() string {
	return filepath.Join(os.Getenv("SystemDrive")+`\`, "DevDrive", "DevDrive.vhdx")
}

// estadoDevDriveScript responde REFS (a letra já é um volume ReFS),
// OCUPADA <tipo> (a letra é outra coisa), VHD (o VHDX existe mas não está
// montado) ou LIVRE.
func estadoDevDriveScript(letra, vhd string) string {
	return fmt.Sprintf(`$v = Get-Volume -DriveLetter %s -ErrorAction SilentlyContinue
if ($v) {
    if ($v.FileSystem -eq 'ReFS') { 'REFS' } else { 'OCUPADA ' + $v.DriveType + ' ' + $v.FileSystem }
} elseif (Test-Path -LiteralPath %s) { 'VHD' } else { 'LIVRE' }`, letra, runner.PSQuote(vhd))
}

// criarDevDriveScript cria o VHDX dinâmico com o diskpart (o New-VHD
// depende do módulo do Hyper-V, que não existe no Home), anexa, particiona
// e formata como Dev Drive.
func criarDevDriveScript(letra, vhd string, gb int) string {
	return fmt.Sprintf(`$vhd = %s
New-Item -ItemType Directory -Force -Path (Split-Path $vhd) | Out-Null
$roteiro = Join-Path $env:TEMP ('devdrive-' + [guid]::NewGuid().ToString('N') + '.txt')
Set-Content -Path $roteiro -Encoding ASCII -Value ('create vdisk file="' + $vhd + '" maximum=%d type=expandable')
try {
    diskpart /s $roteiro
    if ($LASTEXITCODE -ne 0) { throw "diskpart terminou com código $LASTEXITCODE" }
} finally {
    Remove-Item $roteiro -Force -ErrorAction SilentlyContinue
}
$disco = Mount-DiskImage -ImagePath $vhd -NoDriveLetter -PassThru | Get-Disk
Initialize-Disk -Number $disco.Number -PartitionStyle GPT
New-Partition -DiskNumber $disco.Number -UseMaximumSize -DriveLetter %s | Out-Null
Format-Volume -DriveLetter %s -DevDrive -NewFileSystemLabel 'DevDrive' -Confirm:$false -Force | Out-Null`,
		runner.PSQuote(vhd), gb*1024, letra, letra)
}

// montarDevDriveScript reanexa o VHDX e, se a partição voltar sem letra (ou
// com outra), devolve a letra configurada.
func montarDevDriveScript(letra, vhd string) string {
	return fmt.Sprintf(`$img = Get-DiskImage -ImagePath %s
if (-not $img.Attached) { $img = Mount-DiskImage -ImagePath %s -PassThru }
$part = $img | Get-Disk | Get-Partition | Where-Object { $_.Type -eq 'Basic' } | Select-Object -First 1
if ($part.DriveLetter -ne '%s') { $part | Set-Partition -NewDriveLetter %s }`,
		runner.PSQuote(vhd), runner.PSQuote(vhd), letra, letra)
}

// tarefaDevDriveScript registra (ou atualiza) a tarefa que anexa o VHDX no
// boot, como SYSTEM, antes do logon.
func tarefaDevDriveScript(vhd string) string {
	montar := fmt.Sprintf(`if (-not (Get-DiskImage -ImagePath %s).Attached) { Mount-DiskImage -ImagePath %s }`,
		runner.PSQuote(vhd), runner.PSQuote(vhd))
	return fmt.Sprintf(`$acao = New-ScheduledTaskAction -Execute 'powershell.exe' -Argument %s
$gatilho = New-ScheduledTaskTrigger -AtStartup
$quem = New-ScheduledTaskPrincipal -UserId 'SYSTEM' -LogonType ServiceAccount -RunLevel Highest
Register-ScheduledTask -TaskName %s -Action $acao -Trigger $gatilho -Principal $quem -Description 'Monta o Dev Drive (post-windows)' -Force | Out-Null`,
		runner.PSQuote("-NoProfile -NonInteractive -WindowStyle Hidden -Command \""+montar+"\""), runner.PSQuote(tarefaDevDrive))
}

// configurarPastasDevDrive cria Projects e os caches no Dev Drive e aponta
// as ferramentas para eles.
func configurarPastasDevDrive(c *Ctx, raiz string) error {
	caches := filepath.Join(raiz, "Caches")
	vars := []struct{ nome, pasta string }{
		{"npm_config_cache", "npm"},
		{"COMPOSER_CACHE_DIR", "composer"},
		{"PIP_CACHE_DIR", "pip"},
	}

	// CARGO_HOME/RUSTUP_HOME guardam o toolchain e os binários do Rust (o
	// rustup põe CARGO_HOME\bin no PATH). Só dá para escolher antes de
	// instalar: com o Rust já em ~\.cargo, mudar a variável quebraria a
	// instalação existente.
	home := os.Getenv("USERPROFILE")
	_, errCargo := os.Stat(filepath.Join(home, ".cargo"))
	_, errRustup := os.Stat(filepath.Join(home, ".rustup"))
	switch {
	case os.Getenv("CARGO_HOME") != "":
		// Já configurado (reexecução ou escolha manual): não mexe.
	case os.IsNotExist(errCargo) && os.IsNotExist(errRustup):
		vars = append(vars, struct{ nome, pasta string }{"CARGO_HOME", "cargo"}, struct{ nome, pasta string }{"RUSTUP_HOME", "rustup"})
	default:
		c.Log.Infof("Rust já instalado em %s\\.cargo: mantido lá (mover exigiria reinstalar).", home)
	}

	if err := c.MkdirAll(filepath.Join(raiz, "Projects")); err != nil {
		return err
	}
	for _, v := range vars {
		p := filepath.Join(caches, v.pasta)
		if err := c.MkdirAll(p); err != nil {
			return err
		}
		if err := c.SetUserEnv(v.nome, p); err != nil {
			return err
		}
	}

	antiga := filepath.Join(home, "Projects")
	if entradas, err := os.ReadDir(antiga); err == nil && len(entradas) > 0 {
		c.Log.Warnf("%s tem conteúdo: mova os projetos para %sProjects quando puder (não movo sozinho).", antiga, raiz)
	}
	return nil
}

// projetosDir é onde fica a pasta de projetos: no Dev Drive, se ele foi
// criado agora ou numa execução anterior; senão, em %USERPROFILE%\Projects.
func (c *Ctx) projetosDir() string {
	if c.projetos != "" {
		return c.projetos
	}
	if c.DevDriveLetter != "" {
		p := c.DevDriveLetter + `:\Projects`
		if _, err := os.Stat(p); err == nil {
			if fs, err := c.PSOutput(fmt.Sprintf(`(Get-Volume -DriveLetter %s).FileSystem`, c.DevDriveLetter)); err == nil && fs == "ReFS" {
				c.projetos = p
				return p
			}
		}
	}
	return filepath.Join(os.Getenv("USERPROFILE"), "Projects")
}

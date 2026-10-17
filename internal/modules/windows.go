package modules

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vinicgobbi/post-windows/internal/runner"
	"github.com/vinicgobbi/post-windows/internal/winapi"
)

const (
	keyExplorerAdv = `HKCU\Software\Microsoft\Windows\CurrentVersion\Explorer\Advanced`
	keyPersonalize = `HKCU\Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`
	keySearch      = `HKCU\Software\Microsoft\Windows\CurrentVersion\Search`
	keyRun         = `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`
)

var customizacaoWindows = Module{
	ID:    "customizacao_windows",
	Title: "Customização visual do Windows",
	Desc:  "Tema escuro, mostrar extensões de arquivo, menu Iniciar centralizado, ocultar a barra de pesquisa, desativar as recomendações do menu Iniciar (Widgets fica) e Explorer abrindo em \"Este Computador\", sem recentes/frequentes no Acesso Rápido",
	Run: func(c *Ctx) error {
		c.Log.Infof("Aplicando tema escuro e ajustes do Explorer...")
		err := step(
			// EnableTransparency = 1 só reforça que os efeitos de
			// transparência/blur continuam ligados: este programa nunca os
			// desliga.
			func() error {
				return c.SetDWords(keyPersonalize, map[string]uint32{
					"AppsUseLightTheme": 0, "SystemUsesLightTheme": 0, "EnableTransparency": 1,
				})
			},
			// TaskbarAl (menu centralizado) só faz efeito no Windows 11.
			// Start_TrackDocs/Start_IrisRecommendations são os toggles por
			// usuário que ainda existem por baixo da política de
			// "Recomendado".
			//
			// LaunchTo = 1: o Explorer abre em "Este Computador" em vez de
			// "Início", que carrega arquivos recentes e itens da nuvem.
			func() error {
				return c.SetDWords(keyExplorerAdv, map[string]uint32{
					"HideFileExt": 0, "TaskbarAl": 1, "Start_TrackDocs": 0, "Start_IrisRecommendations": 0,
					"LaunchTo": 1,
				})
			},
			// Sem "arquivos recentes" e "pastas frequentes" no Acesso Rápido
			// (os atalhos fixados, como Projects, continuam).
			func() error {
				return c.SetDWords(`HKCU\Software\Microsoft\Windows\CurrentVersion\Explorer`, map[string]uint32{
					"ShowRecent": 0, "ShowFrequent": 0,
				})
			},
			// Esconde a pesquisa da barra de tarefas (o Widgets fica).
			func() error { return c.SetDWord(keySearch, "SearchboxTaskbarMode", 0) },
			func() error {
				return c.SetDWord(`HKLM\SOFTWARE\Policies\Microsoft\Windows\Explorer`, "HideRecommendedSection", 1)
			},
		)
		if err != nil {
			return err
		}

		// Avisa o shell da troca de tema pelo mesmo mecanismo do app
		// Configurações, em vez de depender só do reinício forçado do
		// Explorer: matar o explorer sem isso pode deixar a composição
		// (blur/transparência) inconsistente até o próximo logon.
		if !c.DryRun {
			winapi.BroadcastSettingChange("ImmersiveColorSet")
		}

		// O SearchHost é quem desenha a caixa de pesquisa da barra de
		// tarefas (só reiniciar o Explorer não basta: o valor antigo volta
		// do cache dele); o StartMenuExperienceHost aplica a política de
		// "Recomendado" já nesta sessão.
		for _, p := range []string{"explorer.exe", "SearchHost.exe", "StartMenuExperienceHost.exe"} {
			c.Try("taskkill", "/F", "/IM", p)
		}
		if !c.DryRun {
			reabrirExplorer(c)
		}
		c.Log.Successf("Tema escuro (transparência mantida), extensões de arquivo visíveis, menu centralizado, pesquisa oculta, recomendações do menu Iniciar desativadas e Explorer em \"Este Computador\".")
		return nil
	},
}

// reabrirExplorer espera o Windows reiniciar o shell sozinho (AutoRestartShell)
// e, se isso não acontecer, abre o explorer.exe sem elevação — um explorer
// aberto por este processo herdaria o token de Administrador.
func reabrirExplorer(c *Ctx) {
	for range 6 {
		time.Sleep(500 * time.Millisecond)
		if out, _ := c.Output("tasklist", "/FI", "IMAGENAME eq explorer.exe", "/NH"); strings.Contains(strings.ToLower(out), "explorer.exe") {
			return
		}
	}
	if _, err := c.Deelevated(filepath.Join(os.Getenv("WINDIR"), "explorer.exe")); err != nil {
		c.Log.Warnf("Não consegui reabrir o Explorer (%v); faça logoff e logon para voltar a barra de tarefas.", err)
	}
}

var performance = Module{
	ID:    "performance",
	Title: "Otimizações de desempenho",
	Desc:  "Fixa a pasta Projects no Acesso Rápido, Delivery Optimization, Storage Sense, desativa apps na inicialização, limita a CPU da verificação do Defender, Edge/Chrome sem rodar em segundo plano, desliga telemetria (DiagTrack) e apps em segundo plano, NTFS sem timestamp de acesso, TRIM do SSD, Defender/indexação para pastas de dev, modo de energia \"Melhor desempenho\" na tomada e hibernação (desktop) — sem mexer em efeitos visuais",
	Run: func(c *Ctx) error {
		c.Log.Infof("Aplicando otimizações de desempenho (sem mexer em efeitos visuais)...")
		err := step(
			// Delivery Optimization: P2P de updates só na rede local (por
			// padrão pode enviar/baixar de PCs da internet inteira).
			func() error {
				return c.SetDWord(`HKLM\SOFTWARE\Policies\Microsoft\Windows\DeliveryOptimization`, "DODownloadMode", 1)
			},
			// Storage Sense: liga o interruptor mestre (limpa temporários
			// quando o disco está enchendo).
			func() error {
				return c.SetDWord(`HKCU\SOFTWARE\Microsoft\Windows\CurrentVersion\StorageSense\Parameters\StoragePolicy`, "01", 1)
			},
			// Edge: sem pré-carregamento (Startup Boost) e, como o Chrome,
			// sem continuar rodando em segundo plano depois de fechado
			// (extensões e apps da web mantêm processos vivos sem janela).
			func() error {
				return c.SetDWords(`HKLM\SOFTWARE\Policies\Microsoft\Edge`, map[string]uint32{
					"StartupBoostEnabled": 0, "BackgroundModeEnabled": 0,
				})
			},
			func() error { return c.SetDWord(`HKLM\SOFTWARE\Policies\Google\Chrome`, "BackgroundModeEnabled", 0) },
			// Apps em segundo plano: política "negar à força" (2) + o
			// toggle legado, para builds mais antigas.
			func() error {
				return c.SetDWord(`HKLM\SOFTWARE\Policies\Microsoft\Windows\AppPrivacy`, "LetAppsRunInBackground", 2)
			},
			func() error {
				return c.SetDWord(`HKCU\Software\Microsoft\Windows\CurrentVersion\BackgroundAccessApplications`, "GlobalUserDisabled", 1)
			},
		)
		if err != nil {
			return err
		}

		if err := desativarInicializacao(c); err != nil {
			return err
		}

		// Telemetria: "Connected User Experiences and Telemetry" e "WAP Push
		// Message Routing" geram I/O de disco/rede periódico sem efeito
		// percebido no dia a dia.
		for _, s := range []string{"DiagTrack", "dmwappushsvc"} {
			if c.Probe("sc.exe", "query", s) {
				c.Try("sc.exe", "stop", s)
				c.Try("sc.exe", "config", s, "start=", "disabled")
			}
		}

		// NTFS sem timestamp de último acesso (I/O a cada arquivo LIDO) e
		// TRIM do SSD garantido (o Windows só manda TRIM a discos que
		// suportam).
		c.Try("fsutil", "behavior", "set", "disablelastaccess", "1")
		c.Try("fsutil", "behavior", "set", "disabledeletenotify", "0")

		if err := pastaProjetos(c); err != nil {
			return err
		}
		// Verificação agendada do Defender com no máximo ~30% de CPU em
		// média (padrão 50%), para pesar menos se rodar com o PC em uso. A
		// proteção em tempo real não muda.
		if err := c.PowerShell(`Set-MpPreference -ScanAvgCPULoadFactor 30`); err != nil {
			if c.Ctx.Err() != nil {
				return err
			}
			c.Log.Warnf("Não consegui limitar a CPU da verificação do Defender: %v", err)
		}

		// Energia: desempenho máximo na tomada, economia na bateria — ajusta
		// o plano ATIVO em vez de trocar para "Alto desempenho" fixo.
		c.Try("powercfg", "/setacvalueindex", "SCHEME_CURRENT", "SUB_PROCESSOR", "PROCTHROTTLEMIN", "100")
		c.Try("powercfg", "/setdcvalueindex", "SCHEME_CURRENT", "SUB_PROCESSOR", "PROCTHROTTLEMIN", "5")
		c.Try("powercfg", "/setactive", "SCHEME_CURRENT")
		notebook := ehNotebook(c)
		modoMelhorDesempenho(c, notebook)

		// O SysMain (Superfetch) fica intocado de propósito: desligar é um
		// mito antigo — ele ajuda no cache de apps mesmo em SSD/NVMe.

		// Hibernação: o hiberfil.sys ocupa vários GB (proporcional à RAM).
		// Só desliga em desktop: notebook depende dela ao fechar a tampa ou
		// com bateria fraca. Desligar também desativa o Fast Startup.
		if notebook {
			c.Log.Infof("Notebook detectado: mantendo a hibernação ligada.")
		} else if c.Try("powercfg", "/hibernate", "off") {
			c.Log.Successf("Hibernação desligada (desktop detectado) — libera o espaço do hiberfil.sys.")
		}

		c.Log.Successf("Otimizações de desempenho aplicadas.")
		return nil
	},
}

// pastaProjetos cria a pasta de projetos (no Dev Drive, se houver; senão
// %USERPROFILE%\Projects), fixa no Acesso Rápido, tira o conteúdo da
// indexação (node_modules, .git, target...) e exclui do Defender as pastas
// de dev pesadas que estiverem fora do Dev Drive — nele, o modo de
// desempenho do Defender faz esse papel sem desligar a verificação.
func pastaProjetos(c *Ctx) error {
	proj := c.projetosDir()
	emDevDrive := !strings.EqualFold(proj, filepath.Join(os.Getenv("USERPROFILE"), "Projects"))
	if err := c.MkdirAll(proj); err != nil {
		return err
	}
	// "PinToHome" é um verbo interno estável (não depende do idioma do
	// Windows, ao contrário do texto "Fixar no Acesso Rápido").
	if err := c.PowerShell(`(New-Object -ComObject Shell.Application).Namespace(` + runner.PSQuote(proj) + `).Self.InvokeVerb('PinToHome')`); err != nil {
		if c.Ctx.Err() != nil {
			return err
		}
		c.Log.Warnf("Não consegui fixar %s no Acesso Rápido.", proj)
	}
	c.Try("attrib", "+I", proj)

	var exclusoes []string
	if !emDevDrive {
		exclusoes = append(exclusoes, runner.PSQuote(proj))
	}
	for _, p := range []string{
		filepath.Join(os.Getenv("USERPROFILE"), ".cargo"),
		filepath.Join(os.Getenv("USERPROFILE"), ".rustup"),
	} {
		if _, err := os.Stat(p); err == nil {
			exclusoes = append(exclusoes, runner.PSQuote(p))
		}
	}
	if len(exclusoes) == 0 {
		return nil
	}
	if err := c.PowerShell("Add-MpPreference -ExclusionPath " + strings.Join(exclusoes, ",")); err != nil {
		if c.Ctx.Err() != nil {
			return err
		}
		c.Log.Warnf("Não consegui adicionar as exclusões do Windows Defender: %v", err)
	}
	return nil
}

const (
	keyStartupRun    = `HKCU\Software\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\Run`
	keyStartupFolder = `HKCU\Software\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\StartupFolder`
)

// inicializacaoDesativada são os apps (instalados pelos outros módulos) que
// se põem para abrir no logon sem necessidade: o usuário abre quando
// precisar. Nome do valor em ...\Run (ou do atalho na pasta Inicializar).
// Logi Options+ e Tailscale ficam de fora: precisam estar rodando.
var inicializacaoDesativada = struct{ run, pasta []string }{
	run:   []string{"Spotify", "Steam", "Discord", "Vesktop", "GalaxyClient", "EpicGamesLauncher", "Telegram Desktop"},
	pasta: []string{"Telegram.lnk"},
}

// desativarInicializacao desativa os itens do mesmo jeito que o
// Gerenciador de Tarefas (aba Aplicativos de inicialização): um valor
// "desativado" em StartupApproved, em vez de apagar a entrada do Run. É
// reversível pelo próprio Gerenciador e continua valendo quando o app
// regrava a entrada dele a cada abertura (o Spotify faz isso). Funciona
// também para apps que ainda não criaram a entrada: o Windows respeita o
// valor quando ela aparecer.
func desativarInicializacao(c *Ctx) error {
	run := inicializacaoDesativada.run
	// O Edge cria uma entrada por perfil (MicrosoftEdgeAutoLaunch_<hash>).
	nomes, _ := winapi.ValueNames(keyRun)
	for _, n := range nomes {
		if strings.HasPrefix(n, "MicrosoftEdgeAutoLaunch_") {
			run = append(run, n)
		}
	}
	valor := startupDesativado(time.Now())
	for _, n := range run {
		if err := c.SetBinary(keyStartupRun, n, valor); err != nil {
			return err
		}
	}
	for _, n := range inicializacaoDesativada.pasta {
		if err := c.SetBinary(keyStartupFolder, n, valor); err != nil {
			return err
		}
	}
	return nil
}

// startupDesativado monta o valor do StartupApproved: 12 bytes, o primeiro
// 0x03 (desativado; 0x02 = ativado) e os 8 últimos o FILETIME de quando foi
// desativado (mostrado pelo Gerenciador de Tarefas).
func startupDesativado(t time.Time) []byte {
	b := make([]byte, 12)
	b[0] = 0x03
	// FILETIME: intervalos de 100 ns desde 1601-01-01 UTC. Calculado a
	// partir da época Unix (time.Duration estoura em ~292 anos).
	ft := uint64(t.UnixNano()/100) + filetimeUnix
	binary.LittleEndian.PutUint64(b[4:], ft)
	return b
}

// filetimeUnix é 1970-01-01 em FILETIME.
const filetimeUnix = 116444736000000000

// overlayMelhorDesempenho é o modo de energia "Melhor desempenho" (o
// controle deslizante de Configurações > Sistema > Energia), documentado
// pela Microsoft para OEMs (customize-power-slider).
const overlayMelhorDesempenho = "ded574b5-45a0-4f42-8737-46345c09c238"

// modoMelhorDesempenho liga o modo "Melhor desempenho". O powercfg aplica o
// modo à fonte de energia atual, então no notebook só é aplicado na tomada,
// para não fixar "Melhor desempenho" na bateria.
func modoMelhorDesempenho(c *Ctx, notebook bool) {
	if notebook {
		// BatteryStatus 2 = ligado na tomada.
		if out, err := c.PSOutput(`(Get-CimInstance -ClassName Win32_Battery | Select-Object -First 1).BatteryStatus`); err != nil || out != "2" {
			c.Log.Infof("Notebook na bateria: o modo \"Melhor desempenho\" não foi aplicado (rode de novo na tomada, ou ajuste em Configurações > Sistema > Energia).")
			return
		}
	}
	if c.Try("powercfg", "/overlaysetactive", overlayMelhorDesempenho) {
		onde := ""
		if notebook {
			onde = " na tomada"
		}
		c.Log.Successf("Modo de energia \"Melhor desempenho\" ativado%s.", onde)
	} else {
		// Só existe no plano Equilibrado; com outro plano ativo (ex.: Alto
		// desempenho), o controle deslizante some.
		c.Log.Warnf("Não consegui ativar o modo de energia \"Melhor desempenho\" (ele só existe no plano Equilibrado).")
	}
}

// ehNotebook usa a presença de bateria — mais confiável que o ChassisType,
// que alguns fabricantes preenchem errado.
func ehNotebook(c *Ctx) bool {
	out, err := c.PSOutput(`[bool](Get-CimInstance -ClassName Win32_Battery -ErrorAction SilentlyContinue)`)
	return err == nil && strings.EqualFold(out, "True")
}

// bloatware é uma lista conservadora — só o que quase ninguém usa depois de
// trocar por VLC/Spotify/Telegram/Office. Ficam de fora de propósito: Teams
// (Chat), Widgets, Vincular ao Celular (YourPhone) e o Outlook novo
// (Microsoft.OutlookForWindows).
var bloatware = []string{
	"Microsoft.MicrosoftOfficeHub",
	"Microsoft.MicrosoftSolitaireCollection",
	"Microsoft.MixedReality.Portal",
	"Microsoft.GetHelp",
	"Microsoft.Getstarted",
	"Microsoft.WindowsFeedbackHub",
	"Microsoft.Print3D",
	"Microsoft.3DBuilder",
	"Microsoft.Microsoft3DViewer",
	"Microsoft.ZuneMusic",
	"Microsoft.ZuneVideo",
	"Microsoft.BingWeather",
	"Microsoft.BingNews",
	"Microsoft.BingSearch",
	"Microsoft.Copilot",
	"Microsoft.549981C3F5F10", // Cortana (é o ID real do pacote na Store)
	"Clipchamp.Clipchamp",
	"Microsoft.SkypeApp",
	"Microsoft.Todos",
	"Microsoft.People",
	"Microsoft.WindowsMaps",
	"Microsoft.WindowsAlarms",
	"Microsoft.WindowsSoundRecorder",
	"Microsoft.Wallet",
	// Correio e Calendário nativos, substituídos pelo Outlook novo.
	"microsoft.windowscommunicationsapps",
	// Família Xbox/Game Bar: sem Game Pass/gravação de clipes; Steam/Heroic
	// já têm overlay próprio.
	"Microsoft.GamingApp",
	"Microsoft.XboxGamingOverlay",
	"Microsoft.XboxIdentityProvider",
	"Microsoft.XboxSpeechToTextOverlay",
	"Microsoft.Xbox.TCUI",
	"Microsoft.XboxGameCallableUI",
}

// tarefasTelemetria rodam periodicamente sem efeito visível; variam por
// versão/edição, então as que não existirem são ignoradas.
var tarefasTelemetria = []string{
	`\Microsoft\Windows\Application Experience\Microsoft Compatibility Appraiser`,
	`\Microsoft\Windows\Application Experience\ProgramDataUpdater`,
	`\Microsoft\Windows\Autochk\Proxy`,
	`\Microsoft\Windows\Customer Experience Improvement Program\Consolidator`,
	`\Microsoft\Windows\Customer Experience Improvement Program\UsbCeip`,
	`\Microsoft\Windows\DiskDiagnostic\Microsoft-Windows-DiskDiagnosticDataCollector`,
	`\Microsoft\Windows\Feedback\Siuf\DmClient`,
	`\Microsoft\Windows\Feedback\Siuf\DmClientOnScenarioDownload`,
}

var debloat = Module{
	ID:    "debloat",
	Title: "Remover bloatware do Windows",
	Desc:  "Remove apps pré-instalados (incl. família Xbox/Game Bar, Cortana, Copilot, Bing Search), desativa o Game DVR (evita o diálogo do ms-gamingoverlay) e desliga apps/anúncios sugeridos, dicas e boas-vindas, busca web e destaques da pesquisa e tarefas de telemetria",
	Run: func(c *Ctx) error {
		c.Log.Infof("Removendo apps pré-instalados não usados...")
		lista := make([]string, len(bloatware))
		for i, b := range bloatware {
			lista[i] = runner.PSQuote(b)
		}
		// -AllUsers às vezes falha com 0x80070002 mesmo com o pacote
		// presente; a remoção só do usuário atual resolve nesse caso. O
		// pacote provisionado é removido para não voltar em contas novas.
		script := fmt.Sprintf(`$ErrorActionPreference = 'Continue'
$provisionados = Get-AppxProvisionedPackage -Online -ErrorAction SilentlyContinue
foreach ($nome in @(%s)) {
    $pacotes = Get-AppxPackage -AllUsers -Name $nome -ErrorAction SilentlyContinue
    foreach ($p in $pacotes) {
        $p | Remove-AppxPackage -AllUsers -ErrorAction SilentlyContinue
        $p | Remove-AppxPackage -ErrorAction SilentlyContinue
        "Removido: $nome"
    }
    $provisionados | Where-Object { $_.DisplayName -eq $nome } |
        Remove-AppxProvisionedPackage -Online -ErrorAction SilentlyContinue | Out-Null
}`, strings.Join(lista, ", "))
		if err := c.PowerShell(script); err != nil {
			return err
		}

		err := step(
			// Game DVR desligado: com a família Xbox/Game Bar removida, jogos
			// em tela cheia abririam um diálogo pedindo o "ms-gamingoverlay".
			func() error {
				return c.SetDWord(`HKCU\SOFTWARE\Microsoft\Windows\CurrentVersion\GameDVR`, "AppCaptureEnabled", 0)
			},
			func() error { return c.SetDWord(`HKCU\System\GameConfigStore`, "GameDVR_Enabled", 0) },
			// Sem reinstalação automática de "apps sugeridos" nem anúncios no
			// menu Iniciar. NÃO usar "DisableWindowsConsumerFeatures": essa
			// política também esconde a página do Vincular ao Celular em
			// Configurações.
			//
			// Os SubscribedContent-* são os toggles de Configurações >
			// Sistema > Notificações > Configurações adicionais e do menu
			// Iniciar: 338389 = "Obter dicas e sugestões ao usar o Windows",
			// 310093 = "Mostrar a experiência de boas-vindas" (após
			// atualizações), 338388 = sugestões no Iniciar. SoftLanding são as
			// dicas sobre recursos novos.
			func() error {
				return c.SetDWords(`HKCU\Software\Microsoft\Windows\CurrentVersion\ContentDeliveryManager`, map[string]uint32{
					"SystemPaneSuggestionsEnabled": 0, "SilentInstalledAppsEnabled": 0,
					"SubscribedContent-338389Enabled": 0, "SubscribedContent-310093Enabled": 0,
					"SubscribedContent-338388Enabled": 0, "SoftLandingEnabled": 0,
				})
			},
			// "Sugerir maneiras de concluir a configuração do dispositivo".
			func() error {
				return c.SetDWord(`HKCU\Software\Microsoft\Windows\CurrentVersion\UserProfileEngagement`, "ScoobeSystemSettingEnabled", 0)
			},
			// Busca do menu Iniciar só local, sem Bing a cada tecla, e sem os
			// "destaques da pesquisa" (conteúdo baixado da internet ao abrir
			// a busca).
			func() error {
				return c.SetDWords(`HKLM\SOFTWARE\Policies\Microsoft\Windows\Windows Search`, map[string]uint32{
					"DisableWebSearch": 1, "ConnectedSearchUseWeb": 0, "EnableDynamicContentInWSB": 0,
				})
			},
			func() error {
				return c.SetDWords(keySearch, map[string]uint32{"BingSearchEnabled": 0, "CortanaConsent": 0})
			},
		)
		if err != nil {
			return err
		}

		for _, t := range tarefasTelemetria {
			if c.Probe("schtasks", "/Query", "/TN", t) {
				c.Try("schtasks", "/Change", "/TN", t, "/Disable")
			}
		}
		c.Log.Successf("Bloatware removido, sugestões/dicas do Windows e busca web desativadas e tarefas de telemetria desligadas.")
		return nil
	},
}

var limpeza = Module{
	ID:    "limpeza",
	Title: "Limpeza final",
	Desc:  "Remove arquivos temporários, versões antigas dos componentes do Windows (DISM), o cache do Delivery Optimization e o do Chocolatey (se instalado)",
	Run: func(c *Ctx) error {
		c.Log.Infof("Limpando arquivos temporários...")
		for _, dir := range []string{os.TempDir(), filepath.Join(os.Getenv("WINDIR"), "Temp")} {
			limparPasta(c, dir)
		}
		if runner.Exists("choco") {
			c.Try("choco", "cache", "remove", "--all")
		}

		// Pacotes de atualização já instalados, guardados para compartilhar
		// com outros PCs.
		if err := c.PowerShell(`Delete-DeliveryOptimizationCache -Force`); err != nil {
			if c.Ctx.Err() != nil {
				return err
			}
			c.Log.Warnf("Não consegui limpar o cache do Delivery Optimization: %v", err)
		}

		// Versões antigas dos componentes que sobram depois do Windows
		// Update. Sem /ResetBase de propósito: as atualizações continuam
		// desinstaláveis.
		c.Log.Infof("Limpando versões antigas dos componentes do Windows (pode levar alguns minutos)...")
		if err := c.Run("Dism.exe", "/Online", "/Cleanup-Image", "/StartComponentCleanup"); err != nil {
			if c.Ctx.Err() != nil {
				return err
			}
			c.Log.Warnf("O DISM não concluiu a limpeza dos componentes: %v", err)
		}
		c.Log.Successf("Limpeza concluída.")
		return nil
	},
}

// limparPasta apaga o conteúdo de dir, menos o WorkDir desta execução
// (apagado no final). Arquivos em uso são ignorados.
func limparPasta(c *Ctx, dir string) {
	entradas, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	if c.DryRun {
		c.Log.Infof("[dry-run] apagar o conteúdo de %s (%d itens)", dir, len(entradas))
		return
	}
	apagados := 0
	for _, e := range entradas {
		p := filepath.Join(dir, e.Name())
		if strings.EqualFold(p, c.WorkDir) {
			continue
		}
		if os.RemoveAll(p) == nil {
			apagados++
		}
	}
	c.Log.Infof("%s: %d de %d itens apagados (os demais estão em uso).", dir, apagados, len(entradas))
}

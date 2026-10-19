# POST — Pós-instalação do Windows com TUI

Executável em Go, com interface interativa no terminal (TUI), para configurar
uma workstation Windows recém-instalada: Windows Update, Chrome, Firefox, Git +
Git Credential Manager, LibreOffice e OnlyOffice,
VSCode, DBeaver, Postman, DevToys, fnm/Node.js, PHP + Composer +
driver SQL Server, Bitwarden, Telegram, Vesktop (Discord + Vencord), Spotify,
WhatsApp, VLC, OBS Studio, Obsidian, Ente Auth, Gaphor, jogos (Steam, Heroic,
Prism Launcher), WinBox, qBittorrent, FileZilla, VirtualBox, Tailscale, Logi
Options+, importação de perfis OpenVPN, Claude Code, Rust (rustup, eza,
topgrade), PowerShell 7 + Oh My Posh, Dev Drive, customização/debloat do
Windows e otimizações de desempenho (sem mexer em efeitos visuais).

Contraparte do [`post-install`](https://github.com/vinicgobbi/post-install)
(Linux), com a mesma arquitetura e a mesma TUI — ver "Limitações conhecidas"
no final para as diferenças reais do Windows.

## Requisitos

- Windows 10 2004+ (build 19041) ou Windows 11, **amd64** (única arquitetura
  com build).
- `winget` disponível (App Installer da Microsoft Store atualizado).
- Conta com direitos de Administrador (o programa se reabre pelo UAC).

## Como usar

```powershell
git clone https://github.com/vinicgobbi/post-windows.git
cd post-windows
powershell -ExecutionPolicy Bypass -File .\setup.ps1
```

O `setup.ps1` se reabre como Administrador (UAC), baixa o executável da
última release, confere o SHA-256 e executa apontando `OVPN\` e `logs\` para a
pasta do repositório. Para fixar uma versão:
`$env:POST_WINDOWS_VERSION = 'v1.1.0'; .\setup.ps1`. Qualquer flag é repassada
ao executável (ex.: `.\setup.ps1 --dry-run`). O `-ExecutionPolicy Bypass` é
necessário porque, por padrão, o Windows bloqueia scripts `.ps1`.

### Pacote da release (sem clonar o repositório)

Cada release publica um `.zip` com o executável e a estrutura de pastas:

```
post-windows-windows-amd64.zip
└── post-windows\
    ├── post-windows.exe  # executável
    ├── OVPN\README.md    # coloque aqui os .ovpn a importar
    ├── logs\             # logs de cada execução
    └── README.md
```

Extraia, copie os `.ovpn` para `OVPN\` (opcional) e rode o
`post-windows.exe` num terminal (Windows Terminal, PowerShell ou cmd). Sem
elevação, ele se reabre pelo UAC numa janela nova, com as mesmas flags; essa
janela espera um Enter antes de fechar, para dar tempo de ler o resumo.

Por padrão o executável procura `OVPN\` e grava em `logs\` **ao lado dele
mesmo**, então funciona a partir de qualquer pasta. A release também traz o
`.exe` solto (`post-windows-windows-amd64.exe`, usado pelo `setup.ps1`) e um
`.sha256` para cada arquivo.

### Fluxo da TUI

1. Mostra o sistema detectado (edição, versão, build, arquitetura) e o
   usuário. Diferente do Linux, não há pergunta de usuário: a elevação pelo
   UAC não troca de conta, então a configuração vale sempre para quem
   executou o programa.
2. Checklist com todos os módulos, já marcados:
   - `↑`/`↓` (ou `j`/`k`) — mover · `espaço` — marcar/desmarcar · `a` — todos
   - o painel abaixo mostra a descrição do módulo e as dependências
   - marcar um módulo marca as dependências dele; desmarcar uma dependência
     desmarca quem depende dela (com aviso na tela)
3. Confirmação com a lista do que vai rodar.
4. Execução: barra de progresso, status de cada módulo (✓ ok, ! com avisos,
   ✗ falhou, ⤼ pulado) e a saída ao vivo dos comandos (`↑`/`↓`/`pgup`/`pgdn`
   para rolar, `G` para voltar a seguir). `Ctrl+C` pede confirmação e então
   encerra o comando em andamento (com a árvore de processos dele).
5. Resumo final com o tempo de cada módulo e o caminho dos logs (o resumo
   também fica impresso no terminal depois que a TUI fecha).

Se um módulo falha, os seguintes continuam rodando — exceto os que dependem
dele, que são pulados. Um app que não instala (dentro de um módulo com vários
apps) vira aviso, não falha: os demais apps do módulo continuam. Use
`--fail-fast` para parar tudo no primeiro módulo com erro.

### Flags

| Flag | Descrição |
|------|-----------|
| `--modules a,b` | seleciona só esses módulos (IDs de `--list`) |
| `--skip a,b` | desmarca esses módulos |
| `--dry-run` | simula: lista comandos/arquivos/registro sem alterar nada (não exige Administrador) |
| `--no-tui` | saída em texto simples |
| `--yes` | não pede confirmação (modo texto) |
| `--fail-fast` | para no primeiro módulo que falhar |
| `--list` | lista os módulos e sai |
| `--devdrive-letter X` / `--devdrive-size GB` | letra e tamanho máximo do Dev Drive (padrão `D` e `150`; mínimo 50 GB) |
| `--ovpn-dir` / `--log-dir` | pastas de `.ovpn` e de logs (padrão: `OVPN\` e `logs\` ao lado do executável) |
| `--version` | mostra a versão |

Sem terminal interativo (pipe, CI), o programa usa o modo texto
automaticamente e roda todos os módulos (menos os de `--skip`).

## Módulos

| ID | O que faz |
|----|-----------|
| `bootstrap` | atualiza as fontes do winget e prepara NuGet/PSGallery |
| `windows_update` | PSWindowsUpdate + atualizações pendentes (sem reiniciar; avisa no final) |
| `dev_drive` | Dev Drive (VHDX dinâmico em ReFS) para Projects e os caches de npm/Composer/pip/Cargo — ver abaixo |
| `navegador_git` | Chrome, Firefox, Git, Git Credential Manager; Git com FSMonitor, cache de não rastreados e caminhos longos (também liberados no Windows) |
| `office` | LibreOffice e OnlyOffice Desktop Editors |
| `dev_tools` | VSCode (com "Abrir com o Code" no Explorer), DBeaver, Postman, DevToys, fnm + Node.js LTS |
| `php_sqlsrv` | PHP 8.5, Composer (instalador verificado por SHA-384), ODBC 18, `sqlsrv`/`pdo_sqlsrv` |
| `bitwarden` | Bitwarden |
| `comunicacao_midia` | Telegram, Vesktop, Spotify, WhatsApp, VLC, OBS Studio |
| `produtividade` | Obsidian, Ente Auth, Gaphor |
| `jogos` | Steam, Heroic (Epic/GOG/Amazon), Prism Launcher |
| `utilitarios_rede` | WinBox, qBittorrent, FileZilla (Chocolatey) |
| `virtualbox` | VirtualBox (avisa se VT-x/AMD-V estiver desligado ou o Hyper-V ligado) |
| `tailscale` | Tailscale |
| `logitech_options` | Logi Options+ (equivalente ao Solaar) |
| `ovpn` | OpenVPN GUI + perfis de `OVPN\` |
| `claude_code` | Claude Code |
| `rust_tools` | rustup, eza, topgrade (binários nativos) |
| `powershell_profile` | PowerShell 7 + Oh My Posh com a config do repositório de dotfiles + Nerd Font (requer `navegador_git`) |
| `customizacao_windows` | tema escuro, extensões de arquivo, menu centralizado, sem pesquisa na barra, sem "Recomendado", Explorer em "Este Computador" e sem recentes/frequentes |
| `performance` | pasta Projects no Acesso Rápido, Delivery Optimization, Storage Sense, apps na inicialização desativados, Edge/Chrome sem segundo plano, verificação do Defender limitada a 30% de CPU, apps em segundo plano só do usuário (sem afetar o Family Safety), NTFS/TRIM, Defender/indexação, modo "Melhor desempenho" na tomada, hibernação (só desktop) |
| `debloat` | remove apps pré-instalados, Game DVR, sugestões, dicas e boas-vindas, busca web, destaques da pesquisa e tarefas de telemetria |
| `limpeza` | temporários, componentes antigos do Windows (DISM, sem `/ResetBase`), cache do Delivery Optimization e do Chocolatey |

### Dev Drive

O módulo `dev_drive` cria um [Dev Drive](https://learn.microsoft.com/en-us/windows/dev-drive/):
um volume ReFS feito para código e pacotes. Nele, o Defender verifica os
arquivos de forma assíncrona (modo de desempenho) em vez de segurar cada
abertura — o que faz diferença em `npm install`, `composer install`,
`cargo build` e `git` em repositórios grandes — e o ReFS copia arquivos por
referência (block cloning).

- É um VHDX **dinâmico** em `%SystemDrive%\DevDrive\DevDrive.vhdx`: ocupa só
  o que tiver dentro, até `--devdrive-size` (padrão 150 GB). É criado com o
  `diskpart` (o `New-VHD` depende do Hyper-V, que não existe no Home).
- Uma tarefa agendada (`PostWindows-DevDrive`, como SYSTEM, no boot) remonta
  o VHDX, que não volta sozinho depois de reiniciar.
- `Projects` passa a ficar em `D:\Projects` (fixada no Acesso Rápido) e sem
  exclusão do Defender: o modo de desempenho faz esse papel sem desligar a
  verificação. Se `%USERPROFILE%\Projects` já tiver conteúdo, o módulo avisa
  em vez de mover.
- Caches apontados para `D:\Caches\` (variáveis do usuário):
  `npm_config_cache`, `COMPOSER_CACHE_DIR`, `PIP_CACHE_DIR` e, se o Rust ainda
  não estiver instalado, `CARGO_HOME`/`RUSTUP_HOME` — o módulo roda antes
  dos que instalam essas ferramentas. Com o Rust já em `~\.cargo`, ele fica
  lá (mover exigiria reinstalar).
- Nada no Dev Drive é indexado pelo Windows Search, nem nomes nem conteúdo:
  a política "Impedir a indexação de determinados caminhos" (`file:///D:\*`)
  tira a unidade da indexação e impede que ela seja incluída de novo pelas
  Configurações. A pasta de projetos fica fora da indexação da mesma forma
  mesmo sem Dev Drive (em `%USERPROFILE%\Projects`).
- A unidade inteira fica acessível só para o seu usuário (além do SYSTEM e
  dos Administradores): num PC compartilhado, uma conta padrão — como a de
  um filho no Family Safety — não vê nem apaga nada nela. Isso só protege
  enquanto a outra conta for de "Usuário padrão": um administrador consegue
  tomar posse de qualquer pasta.
- Se a letra já estiver em uso por outra unidade, o módulo avisa e pula (use
  `--devdrive-letter`). Exige Windows 11 22H2 (build 22621.2338) ou mais novo.

### Family Safety

As otimizações não atrapalham o controle parental da Microsoft numa conta de
filho no mesmo PC: os apps em segundo plano são desligados só para quem roda
o programa (não pela política de máquina, que pegaria o app Family Safety), e
o serviço de telemetria (DiagTrack) fica no padrão do Windows — há relatos de
que o relatório de atividade para de chegar sem ele. Se uma execução anterior
tinha aplicado a política ou desligado o serviço, o módulo `performance`
desfaz isso.

### PHP: php.ini fora da pasta do pacote

O winget recria a pasta do PHP a cada atualização, levando junto o
`php.ini` e as DLLs postas ali. Por isso o módulo `php_sqlsrv` guarda tudo
numa pasta fixa, `%ProgramData%\PHP\8.5\`:

- `php.ini` — o PHP é apontado para ele pelo valor `IniFilePath` de
  `HKLM\SOFTWARE\PHP\8.5`, que ele consulta antes da própria pasta. A chave é
  por versão, então não afeta outro PHP instalado lado a lado (a variável
  `PHPRC` valeria para todos);
- `ext\php_sqlsrv_85_ts*.dll` e `ext\php_pdo_sqlsrv_85_ts*.dll` —
  carregadas pelo caminho absoluto no `php.ini`.

As extensões que vêm com o PHP (openssl, curl, mbstring...) continuam com
`extension_dir = "ext"` relativo, que o Windows resolve a partir da pasta do
`php.exe` — ou seja, acompanha a pasta nova depois de uma atualização. Em
reexecuções, o `php.ini` da pasta fixa é preservado (edições à mão ficam); um
`php.ini` antigo na pasta do pacote é migrado na primeira vez. No final, o
módulo confere com o próprio `php` qual `php.ini` foi carregado e quais
extensões subiram.

### winget e Chocolatey

O `winget` é o gerenciador principal. Os apps já instalados são detectados
de uma vez com `winget export` (a tabela do `winget list` trunca IDs longos),
e os códigos de saída próprios do winget são tratados: "já instalado" conta
como sucesso, e "reinicie para concluir" vira aviso. Quando um app não tem
pacote no winget (ex.: FileZilla) ou a instalação falha, o programa cai para
o Chocolatey, instalado sob demanda só na primeira vez que for preciso.

Spotify e WhatsApp falham quando o winget roda elevado; para eles o programa
cria uma tarefa agendada temporária com `RunLevel Limited`, que roda o
`winget.exe` (pelo caminho real, já que o alias `winget` não resolve numa
tarefa agendada) como o usuário sem elevação, e copia a saída para o log.

## Log em arquivo

Cada execução grava dois arquivos em `logs\`:

- `setup_<timestamp>.log` — só as mensagens (info/aviso/erro/sucesso/passo),
  com hora e nível. Bom para uma leitura rápida.
- `setup_<timestamp>.raw.log` — transcrição completa, incluindo cada comando,
  cada valor de registro gravado e a saída crua de
  `winget`/`choco`/`powershell`. Bom para investigar uma falha.

As animações do winget (spinner e barras de progresso) ficam só no
`.raw.log`, para não inundar a tela.

## Estrutura do projeto

```
setup.ps1                    # bootstrap: eleva, baixa o .exe da release e executa
cmd/post-windows/main.go     # flags, elevação (UAC), TUI x modo texto
internal/
  sysinfo/                   # edição/versão/build do Windows (registro)
  winapi/                    # só-Windows: registro, UAC, PATH, WM_SETTINGCHANGE, árvore de processos
  runner/                    # comandos, PowerShell (-EncodedCommand), execução sem elevação,
                             #   registro, arquivos, downloads, dry-run
  runlog/                    # logs .log/.raw.log e repasse das mensagens à interface
  engine/                    # dependências entre módulos e execução em ordem
  modules/                   # os módulos
    module.go                #   tipo Module, ordem de execução (All) e helpers
    winget.go                #   instalação via winget/Chocolatey, detecção de instalados
    apps.go                  #   bootstrap, Windows Update, apps, VirtualBox, Tailscale, OpenVPN
    devdrive.go              #   Dev Drive (VHDX ReFS), tarefa de montagem, caches de pacotes
    office.go                #   LibreOffice e OnlyOffice
    php.go                   #   PHP, php.ini, Composer, sqlsrv/pdo_sqlsrv
    perfil.go                #   PowerShell 7 + Oh My Posh, perfis 5.1/7
    windows.go               #   customização, desempenho, debloat, limpeza
  config/apps.go             # catálogo de apps (IDs do winget)
  tui/                       # interface Bubble Tea
  plain/                     # saída em texto simples
.github/workflows/release.yml
.cz.toml                     # configuração do commitizen
```

O pacote `winapi` tem stubs para outros sistemas, então `go vet` e
`go test ./...` também rodam no Linux; o CI roda os testes no Windows.

## Adicionando ou editando um módulo

1. Para só instalar apps, acrescente-os ao catálogo em
   `internal/config/apps.go` (ou crie uma lista nova e um módulo com
   `moduloApps(...)` em `internal/modules/apps.go`).
2. Para algo diferente, crie uma variável `Module{ID, Title, Desc, Deps, Run}`
   no arquivo do grupo e coloque-a em `All()` (`internal/modules/module.go`)
   na posição de execução desejada.
3. Use os helpers do `Ctx`: `installApp`/`installApps`, `Run`/`Try`/`Probe`/
   `Output`, `PowerShell`/`PSOutput`, `SetDWord`/`SetDWords`/`DeleteValue`,
   `WriteFile`/`AppendLine`/`CopyFile`, `Download`/`Get`. Todos respeitam o
   `--dry-run` e mandam a saída para o log.

## Desenvolvimento

```bash
go test ./...
GOOS=windows GOARCH=amd64 go vet ./...
GOOS=windows GOARCH=amd64 go build -o post-windows.exe ./cmd/post-windows
```

As releases seguem o mesmo fluxo do post-install: a cada push na `main`, a
action roda os testes no Windows, o commitizen calcula a versão pelos
Conventional Commits e, se houver `feat`/`fix`, publica a release.

## Limitações conhecidas

- **Sem Flatpak**: a maioria dos apps é instalador nativo (exe/msi) via
  winget; manifests da comunidade têm qualidade variável — se um app parar de
  instalar silenciosamente, é o primeiro lugar para investigar (`.raw.log`).
- **Reboots**: o Windows Update instala tudo com `-IgnoreReboot` de propósito
  (para não derrubar os módulos seguintes); reinicie ao final.
- **UAC residual**: mesmo elevado, alguns instaladores ainda abrem o próprio
  prompt — não é 100% garantido zero interação.
- **Edição do Windows**: Hyper-V e `gpedit.msc` só existem em
  Pro/Enterprise/Education — por isso o VirtualBox em vez do Hyper-V.
- **sqlsrv/pdo_sqlsrv**: as DLLs vêm da release mais recente do
  [`microsoft/msphpsql`](https://github.com/microsoft/msphpsql); se ela ainda
  não tiver build para o PHP 8.5, o módulo avisa e pula.
- **Saída de ferramentas nativas**: `sc.exe`, `fsutil` e `powercfg` escrevem
  na página de código OEM, então acentos podem aparecer trocados no
  `.raw.log` (o conteúdo não é afetado).

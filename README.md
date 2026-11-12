# POST — Pós-instalação do Windows com TUI

Executável em Go, com interface interativa no terminal (TUI), para configurar
uma workstation Windows recém-instalada: Windows Update, Dev Drive,
ferramentas de terminal (GitHub CLI, Gitea CLI, bat, btop4win, FFmpeg, Nmap,
Python + pipx), PowerShell 7 + Oh My Posh com os dotfiles pessoais, Chrome,
Firefox, Bitwarden, os aplicativos (Postman, DBeaver, DevToys, Obsidian, Ente
Auth, Gaphor, LibreOffice, OnlyOffice, Telegram, Vesktop, Spotify, WhatsApp,
VLC, OBS Studio, qBittorrent, FileZilla, WinBox), jogos (Steam, Heroic, Prism
Launcher), Logi Options+, Git + Git Credential Manager, VSCode, Go, mise com
Node + pnpm e Rust (eza, topgrade), PHP + Composer + driver SQL Server, Ruby
(Bundler e Jekyll), Claude Code, deploy-dev, Tailscale, importação de perfis
OpenVPN, VirtualBox, papel de parede, customização do Windows e
otimizações de desempenho (sem mexer em efeitos visuais).

Contraparte do [`post-install`](https://github.com/vinicgobbi/post-install)
(Linux), com a mesma arquitetura, a mesma TUI e os mesmos IDs de módulo
quando o módulo faz a mesma coisa — veja
[Correspondência com o post-install](#correspondência-com-o-post-install).

## Sistemas suportados

- **Windows 11** nas versões com suporte da Microsoft para as edições
  Home/Pro — hoje **25H2 (build 26200), 26H1 (28000) e 26H2 (26300)**;
- **amd64** (única arquitetura com build);
- `winget` disponível (App Installer da Microsoft Store atualizado).

A lista de versões fica em
[`internal/sysinfo/versoes.go`](internal/sysinfo/versoes.go) e é mantida
pelo workflow `versoes.yml` (veja [Versões suportadas](#versões-suportadas)).

- **Builds mais novos que os da lista** (ex.: um Insider, ou uma versão que
  saiu depois da release do post-windows) podem funcionar, mas não foram
  testados: o programa avisa e pede confirmação antes de seguir (na TUI, uma
  tela com "Sair" como padrão; no modo texto, `[s/N]`). Sem terminal
  interativo, segue só com `--allow-untested-version` ou `--yes`; no
  `--dry-run`, que não altera nada, só avisa.
- **Versões fora de suporte** — o Windows 10 (2004/build 19041 em diante,
  onde o winget existe) e as versões do Windows 11 que saíram da lista —
  continuam funcionando, com um aviso no início: os módulos existem nelas,
  mas elas não recebem mais correções de segurança da Microsoft (nas
  edições Home/Pro). Diferente do post-install, que recusa versões fora de
  suporte, aqui só o que não tem winget (antes do build 19041) é recusado.
- O Dev Drive exige o Windows 11 22H2 (build 22621.2338) ou mais novo; nos
  anteriores o módulo `dev_drive` avisa e pula.

## Correspondência com o post-install

Os IDs são os mesmos quando o módulo faz a mesma coisa nas duas
ferramentas, então `--modules`/`--skip` valem igual:

| ID | No Windows |
|----|------------|
| `atualizacao` / `limpeza` | Windows Update (PSWindowsUpdate) / temporários, DISM e caches |
| `ferramentas` | os mesmos CLIs pelo winget (btop4win no lugar do bashtop; Nmap pelo Chocolatey); sem jq e FUSE |
| `shell` | PowerShell 7 + Oh My Posh com a config dos mesmos dotfiles, no lugar do Zsh + Oh My Zsh |
| `chrome`, `bitwarden` | instaladores oficiais pelo winget |
| `firefox` | só o Firefox: o Thunderbird fica de fora (o Outlook novo já vem com o Windows) |
| `apps` / `jogos` | o equivalente aos `flatpaks` / `flatpaks_jogos`: os mesmos apps pelo winget (a lista e as exceções estão em [`internal/config/apps.go`](internal/config/apps.go)) |
| `logitech` | Logi Options+, o equivalente ao `solaar` |
| `git_gcm`, `vscode`, `go`, `claude_code`, `deploy_dev` | iguais, pelo winget (o VSCode com "Abrir com o Code" no Explorer, o equivalente da extensão do Nautilus) |
| `mise` / `node` / `rust` | mise pelo winget, com os shims no PATH; Node LTS + pnpm e Rust stable pelo mise (eza e topgrade como binários nativos, sem compilar) |
| `php_sqlserver` | PHP 8.5, Composer, ODBC 18 e as DLLs oficiais do sqlsrv/pdo_sqlsrv (veja [PHP](#php-phpini-fora-da-pasta-do-pacote)) |
| `ruby` | RubyInstaller com DevKit (e não o mise: é o caminho estável do Ruby no Windows) |
| `tailscale` | o cliente oficial, que já tem ícone na bandeja (sem Trayscale) |
| `ovpn` | OpenVPN GUI + perfis de `OVPN\` |
| `virtualbox` | o equivalente ao `virt_manager` |
| `papel_parede` | igual, com `Wallpapers\` |

Só existem no Windows: `bootstrap`, `dev_drive`, `customizacao_windows` e
`performance`. Só existem no Linux (sem equivalente útil no
Windows): `mirrors`, `remover_libreoffice`, `podman` (exigiria o WSL2),
`firewall` (o Windows já traz o seu), `temas_yaru`, `extensoes_gnome`,
`atalhos_gnome`, `nautilus` e `apps_gnome`.

**IDs de versões anteriores.** Até a 1.x, o post-windows tinha IDs próprios
(`navegador_git`, `dev_tools`, `php_sqlsrv`, `rust_tools`,
`powershell_profile`...). `--modules` e `--skip` ainda aceitam esses IDs,
com um aviso, e os trocam pelos módulos que fazem o mesmo hoje (a lista
está em `Renomeados`, em [`internal/modules/module.go`](internal/modules/module.go)).
O `debloat`, removido na 2.0, é ignorado com um aviso (`Removidos`, no
mesmo arquivo): o programa não remove mais apps pré-instalados nem mexe
nas sugestões, na busca web, no Game DVR e nas tarefas de telemetria.

## Módulos

Os módulos ficam em grupos, mostrados nessa ordem na TUI e no `--list`, e
rodam de cima para baixo. Cada um é autocontido: instala o que usa, então
pode ser marcado ou desmarcado sozinho.

| Grupo | ID | O que faz |
|-------|----|-----------|
| Sistema | `bootstrap` | atualiza as fontes do winget e prepara NuGet/PSGallery |
| | `atualizacao` | PSWindowsUpdate + atualizações pendentes (sem reiniciar; avisa no final) |
| | `dev_drive` | Dev Drive (VHDX dinâmico em ReFS) para Projects e os caches de npm/Composer/pip/Cargo — veja abaixo |
| Terminal | `ferramentas` | GitHub CLI, Gitea CLI (`tea`), bat, btop4win, FFmpeg, Nmap (Chocolatey), Python 3.14 + pipx |
| | `shell` | PowerShell 7 + Oh My Posh com a config dos dotfiles + Nerd Font |
| Aplicativos | `chrome` | Google Chrome |
| | `firefox` | Firefox |
| | `bitwarden` | Bitwarden |
| | `apps` | Postman, DBeaver, DevToys, Obsidian, Ente Auth, Gaphor, LibreOffice, OnlyOffice, Telegram, Vesktop, Spotify, WhatsApp, VLC, OBS Studio, qBittorrent, FileZilla (Chocolatey), WinBox |
| | `jogos` | Steam, Heroic (Epic/GOG/Amazon), Prism Launcher |
| | `logitech` | Logi Options+ |
| Desenvolvimento | `git_gcm` | Git e Git Credential Manager; Git com FSMonitor, cache de não rastreados e caminhos longos (também liberados no Windows) |
| | `vscode` | VSCode, com "Abrir com o Code" no Explorer |
| | `go` | Go |
| | `mise` | mise, com os shims no PATH do usuário |
| | `node` | Node LTS + pnpm pelo mise |
| | `php_sqlserver` | PHP 8.5, Composer (instalador verificado por SHA-384), ODBC 18, `sqlsrv`/`pdo_sqlsrv` |
| | `rust` | Rust stable pelo mise; eza e topgrade (binários nativos) |
| | `ruby` | Ruby 3.4 com DevKit (`ridk install 3`), Bundler e Jekyll; `bundle install` em `vendor/bundle` |
| | `claude_code` | Claude Code |
| | `deploy_dev` | deploy-dev (última release do deploy-tools, SHA-256 conferido) |
| Rede e virtualização | `tailscale` | Tailscale |
| | `ovpn` | OpenVPN GUI + perfis de `OVPN\` |
| | `virtualbox` | VirtualBox (avisa se VT-x/AMD-V estiver desligado ou o Hyper-V ligado) |
| Windows e aparência | `papel_parede` | aplica o papel de parede escolhido |
| | `customizacao_windows` | tema escuro, extensões de arquivo, menu centralizado, sem pesquisa na barra, sem "Recomendado", Explorer em "Este Computador" e sem recentes/frequentes |
| | `performance` | pasta Projects no Acesso Rápido, Delivery Optimization, Storage Sense, apps na inicialização desativados, Edge/Chrome sem segundo plano, verificação do Defender limitada a 30% de CPU, apps em segundo plano só do usuário (sem afetar o Family Safety), NTFS/TRIM, Defender/indexação, modo "Melhor desempenho" na tomada, hibernação (só desktop) |
| Finalização | `limpeza` | temporários, componentes antigos do Windows (DISM, sem `/ResetBase`), cache do Delivery Optimization e do Chocolatey |

O `dev_drive` roda antes dos módulos que instalam npm, Composer, pip e Rust,
para os caches já nascerem nele. `node` e `rust` dependem do `mise` e o
marcam junto.

## mise (Node, pnpm, Rust)

O módulo `mise` instala o [mise](https://mise.jdx.dev) pelo winget
(`jdx.mise`) e põe a pasta dos shims (`%LOCALAPPDATA%\mise\shims`) no PATH
do usuário — a forma que a documentação do mise recomenda no Windows: vale
em qualquer terminal (PowerShell 5.1 e 7, cmd) e para programas abertos fora
dele (VSCode, tarefas), sem linha nenhuma no perfil.

As ferramentas ficam como globais do usuário em
`~\.config\mise\config.toml` (`mise use -g`): `node@lts` e `pnpm@latest`
(módulo `node`) e `rust@stable` (módulo `rust`). Para atualizar, use
`mise upgrade`. Para fixar outra versão num projeto, use `mise use node@22`
dentro dele, o que grava um `mise.toml` no projeto.

Ao rodar de novo numa máquina configurada por uma versão anterior (que
usava o fnm), o módulo `node` tira dos perfis do PowerShell a ativação do
fnm. O fnm em si continua instalado; para removê-lo:
`winget uninstall Schniz.fnm`. Com o Dev Drive, o Rust usa o `CARGO_HOME` e
o `RUSTUP_HOME` dele (veja [Dev Drive](#dev-drive)).

Para compilar projetos Rust, o toolchain padrão no Windows (MSVC) precisa do
linker do Visual Studio: instale o Visual Studio Build Tools com
"Desenvolvimento para desktop com C++" (o módulo avisa quando ele não está
instalado). eza e topgrade não precisam disso: vêm como binários prontos.

## Como usar

```powershell
git clone https://github.com/vinicgobbi/post-windows.git
cd post-windows
powershell -ExecutionPolicy Bypass -File .\setup.ps1
```

O `setup.ps1` se reabre como Administrador (UAC), baixa o executável da
última release, confere o SHA-256 e executa apontando `OVPN\`,
`Wallpapers\` e `logs\` para a pasta do repositório. Para fixar uma versão:
`$env:POST_WINDOWS_VERSION = 'v2.0.0'; .\setup.ps1`. Qualquer flag é
repassada ao executável (ex.: `.\setup.ps1 --dry-run`). O
`-ExecutionPolicy Bypass` é necessário porque, por padrão, o Windows bloqueia
scripts `.ps1`. Como o próprio script já baixa a release, ele roda o
executável com `--no-update-check`.

### Pacote da release (sem clonar o repositório)

Cada release publica um `.zip` com o executável e a estrutura de pastas:

```
post-windows-windows-amd64.zip
└── post-windows\
    ├── post-windows.exe  # executável
    ├── OVPN\README.md    # coloque aqui os .ovpn a importar
    ├── Wallpapers\       # papéis de parede oferecidos na TUI
    ├── logs\             # logs de cada execução
    └── README.md
```

Extraia, copie os `.ovpn` para `OVPN\` (opcional) e rode o
`post-windows.exe` num terminal (Windows Terminal, PowerShell ou cmd). Sem
elevação, ele se reabre pelo UAC numa janela nova, com as mesmas flags; essa
janela espera um Enter antes de fechar, para dar tempo de ler o resumo.

Por padrão o executável procura `OVPN\` e `Wallpapers\` e grava em `logs\`
**ao lado dele mesmo**, então funciona a partir de qualquer pasta. A release
também traz o `.exe` solto (`post-windows-windows-amd64.exe`, usado pelo
`setup.ps1`) e um `.sha256` para cada arquivo.

### Verificando o download

O `.sha256` só prova que o arquivo chegou inteiro (ele vem da mesma
release). Para provar que o executável foi compilado pela action deste
repositório, a partir do código da tag, cada release tem um **atestado de
proveniência** (Sigstore), verificável com o
[GitHub CLI](https://cli.github.com/):

```powershell
gh attestation verify post-windows-windows-amd64.zip --repo vinicgobbi/post-windows
```

Releases anteriores a essa mudança não têm atestado.

### Fluxo da TUI

1. Mostra o sistema detectado (edição, versão, build, arquitetura), o
   usuário e os avisos sobre a versão do Windows. Se houver uma versão mais
   nova do post-windows, a primeira tela oferece atualizar agora ou ignorar
   (veja [Atualização](#atualização)). Diferente do Linux, não há pergunta
   de usuário: a elevação pelo UAC não troca de conta, então a configuração
   vale sempre para quem executou o programa.
2. Pergunta qual papel de parede aplicar, entre as imagens de `Wallpapers\`
   (com resolução e tamanho), ou "Não alterar", que desmarca o módulo
   `papel_parede`. A tela é pulada se a pasta estiver vazia ou se vier
   `--wallpaper`.
3. Checklist com todos os módulos, já marcados:
   - `↑`/`↓` (ou `j`/`k`) — mover · `espaço` — marcar/desmarcar · `a` — todos
   - os módulos aparecem agrupados (veja [Módulos](#módulos)); o painel
     abaixo mostra a descrição do módulo e as dependências
   - marcar um módulo marca as dependências dele; desmarcar uma dependência
     desmarca quem depende dela (com aviso na tela)
4. Confirmação com a lista do que vai rodar (e o papel de parede).
5. Execução: barra de progresso, status de cada módulo (✓ ok, ! com avisos,
   ✗ falhou, ⤼ pulado) e a saída ao vivo dos comandos (`↑`/`↓`/`pgup`/`pgdn`
   para rolar, `G` para voltar a seguir). `Ctrl+C` pede confirmação e então
   encerra o comando em andamento (com a árvore de processos dele).
6. Resumo final com o tempo de cada módulo e o caminho dos logs (o resumo
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
| `--wallpaper NOME` | papel de parede a aplicar (nome do arquivo, ou `none` para não alterar); sem ela, a TUI pergunta |
| `--devdrive-letter X` / `--devdrive-size GB` | letra e tamanho máximo do Dev Drive (padrão `D` e `150`; mínimo 50 GB) |
| `--ovpn-dir` / `--wallpapers-dir` / `--log-dir` | pastas de `.ovpn`, papéis de parede e logs (padrão: `OVPN\`, `Wallpapers\` e `logs\` ao lado do executável) |
| `--no-update-check` | não verifica se há versão nova do post-windows |
| `--allow-untested-version` | segue, sem perguntar, numa versão do Windows mais nova que as testadas (por sua conta e risco) |
| `--version` | mostra a versão |

Sem terminal interativo (pipe, CI), o programa usa o modo texto
automaticamente e roda todos os módulos (menos os de `--skip`).

## Atualização

Ao iniciar, o post-windows consulta a última release no GitHub (com prazo de
5 s; sem rede, só avisa e segue). Se houver uma versão mais nova:

- **TUI** — a primeira tela oferece *Atualizar agora* ou *Ignorar e continuar*;
- **modo texto** — pergunta `Atualizar agora? [S/n]`;
- **`--dry-run`, `--yes` ou sem terminal** — só avisa, sem atualizar.

Atualizar baixa o `.zip` da release, confere o SHA-256 (pelo `.sha256`
publicado e pelo digest que o GitHub calcula) e substitui o executável.
Quando ele roda de dentro do pacote da release (com `Wallpapers\` ao lado),
atualiza também `Wallpapers\`, `README.md` e `OVPN\README.md` — os seus
`.ovpn` e os `logs\` nunca são tocados. O Windows não deixa sobrescrever um
`.exe` em execução, então o atual é renomeado para `post-windows.exe.old` e
apagado na próxima execução. A execução em andamento continua sendo a versão
antiga: ao final, o programa mostra o comando para executar de novo (com as
mesmas opções) e sai.

Builds locais (`go build`, versão `dev`) não verificam atualização, e o
`setup.ps1` desliga a verificação (ele já baixa a release).

## Log em arquivo

Cada execução grava dois arquivos em `logs\`:

- `setup_<timestamp>.log` — só as mensagens (info/aviso/erro/sucesso/passo),
  com hora e nível. Bom para uma leitura rápida.
- `setup_<timestamp>.raw.log` — transcrição completa, incluindo cada comando,
  cada valor de registro gravado e a saída crua de
  `winget`/`choco`/`powershell`. Bom para investigar uma falha.

As animações do winget (spinner e barras de progresso) ficam só no
`.raw.log`, para não inundar a tela.

## Dev Drive

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
  mesmo sem Dev Drive (em `%USERPROFILE%\Projects`), assim como as pastas de
  pacotes, caches e instalações de ferramentas de dev fora do Dev Drive
  (`~\.cargo`, `~\.rustup`, `~\.vscode`, mise, npm, pnpm, Composer, pip,
  Yarn, apps "portable" do winget etc.) — inclusive as que ainda não
  existem, para ferramentas instaladas depois.
- A unidade inteira fica acessível só para o seu usuário (além do SYSTEM e
  dos Administradores): num PC compartilhado, uma conta padrão — como a de
  um filho no Family Safety — não vê nem apaga nada nela. Isso só protege
  enquanto a outra conta for de "Usuário padrão": um administrador consegue
  tomar posse de qualquer pasta.
- Se a letra já estiver em uso por outra unidade, o módulo avisa e pula (use
  `--devdrive-letter`). Exige Windows 11 22H2 (build 22621.2338) ou mais novo.

## Family Safety

As otimizações não atrapalham o controle parental da Microsoft numa conta de
filho no mesmo PC: os apps em segundo plano são desligados só para quem roda
o programa (não pela política de máquina, que pegaria o app Family Safety), e
o serviço de telemetria (DiagTrack) fica no padrão do Windows — há relatos de
que o relatório de atividade para de chegar sem ele. Se uma execução anterior
tinha aplicado a política ou desligado o serviço, o módulo `performance`
desfaz isso.

## PHP: php.ini fora da pasta do pacote

O winget recria a pasta do PHP a cada atualização, levando junto o
`php.ini` e as DLLs postas ali. Por isso o módulo `php_sqlserver` guarda
tudo numa pasta fixa, `%ProgramData%\PHP\8.5\`:

- `php.ini` — o PHP é apontado para ele pelo valor `IniFilePath` de
  `HKLM\SOFTWARE\PHP\8.5`, que ele consulta antes da própria pasta. A chave é
  por versão, então não afeta outro PHP instalado lado a lado (a variável
  `PHPRC` valeria para todos);
- `ext\php_sqlsrv_85_ts*.dll` e `ext\php_pdo_sqlsrv_85_ts*.dll` —
  carregadas pelo caminho absoluto no `php.ini`. O pacote do
  [`microsoft/msphpsql`](https://github.com/microsoft/msphpsql) é conferido
  pelo SHA-256 que o GitHub publica para a release.

As extensões que vêm com o PHP (openssl, curl, mbstring...) continuam com
`extension_dir = "ext"` relativo, que o Windows resolve a partir da pasta do
`php.exe` — ou seja, acompanha a pasta nova depois de uma atualização. Em
reexecuções, o `php.ini` da pasta fixa é preservado (edições à mão ficam); um
`php.ini` antigo na pasta do pacote é migrado na primeira vez. No final, o
módulo confere com o próprio `php` qual `php.ini` foi carregado e quais
extensões subiram.

## winget e Chocolatey

O `winget` é o gerenciador principal. Os apps já instalados são detectados
de uma vez com `winget export` (a tabela do `winget list` trunca IDs longos),
e os códigos de saída próprios do winget são tratados: "já instalado" conta
como sucesso, e "reinicie para concluir" vira aviso. Quando um app não tem
pacote no winget (ex.: FileZilla) ou o pacote de lá está defasado (ex.: Nmap),
ele vai direto pelo Chocolatey; quando a instalação pelo winget falha, o
programa cai para o Chocolatey. O Chocolatey é instalado sob demanda, só na
primeira vez que for preciso.

Spotify e WhatsApp falham quando o winget roda elevado; para eles o programa
cria uma tarefa agendada temporária com `RunLevel Limited`, que roda o
`winget.exe` (pelo caminho real, já que o alias `winget` não resolve numa
tarefa agendada) como o usuário sem elevação, e copia a saída para o log.

## Papel de parede

Os papéis de parede ficam em `Wallpapers\` (`.jpg`, `.jpeg`, `.png` ou
`.bmp`) — para oferecer outra imagem, basta colocá-la na pasta, sem
recompilar. São as mesmas imagens do post-install. O módulo `papel_parede`:

1. copia a imagem escolhida para `%LOCALAPPDATA%\post-windows\Wallpapers\`
   (fora da pasta Imagens, que pode estar no OneDrive: o Windows lê a imagem
   de lá a cada logon);
2. aplica no modo Preencher (`WallpaperStyle` 10, o "zoom" do GNOME) pelo
   mesmo caminho do app Configurações (`SystemParametersInfo`), que já
   atualiza a área de trabalho.

O WebP não entra na lista: o Windows só o aplica como papel de parede com a
"Extensões de Imagem da Web" da Store.

## Estrutura do projeto

```
setup.ps1                    # bootstrap: eleva, baixa o .exe da release e executa
cmd/post-windows/main.go     # flags, elevação (UAC), TUI x modo texto
internal/
  sysinfo/                   # edição/versão/build do Windows (registro) e checagem de suporte (versoes.go)
  winapi/                    # só-Windows: registro, UAC, PATH, papel de parede, WM_SETTINGCHANGE, árvore de processos
  winpath/                   # caminhos do Windows tratados igual em qualquer sistema (para os testes golden)
  runner/                    # comandos, PowerShell (-EncodedCommand), execução sem elevação,
                             #   registro, arquivos, downloads (SHA-256), dry-run, máquina falsa (host.go)
  runlog/                    # logs .log/.raw.log e repasse das mensagens à interface
  engine/                    # dependências entre módulos e execução em ordem
  modules/                   # os módulos (um arquivo por grupo)
    module.go                #   tipo Module, grupos, ordem de execução (All), IDs renomeados e helpers
    pins.go                  #   commit fixado do repositório de dotfiles
    winget.go                #   instalação via winget/Chocolatey, detecção de instalados
    sistema.go               #   Sistema e Finalização: bootstrap, Windows Update, limpeza
    devdrive.go              #   Dev Drive (VHDX ReFS), tarefa de montagem, caches de pacotes
    terminal.go              #   Terminal: ferramentas, PowerShell 7 + Oh My Posh, perfis 5.1/7
    apps.go                  #   Aplicativos: Chrome, Firefox, Bitwarden, apps, jogos, Logi Options+
    dev.go                   #   Desenvolvimento: Git/GCM, VSCode, Go, mise, Node, Rust, Ruby, Claude Code, deploy-dev
    php.go                   #   PHP, php.ini, Composer, sqlsrv/pdo_sqlsrv
    rede.go                  #   Rede e virtualização: Tailscale, OpenVPN, VirtualBox
    wallpaper.go             #   papel de parede
    windows.go               #   customização e desempenho
    golden_test.go           #   testes golden do dry-run (cenários em testdata/golden/)
  config/apps.go             # catálogo de apps (IDs do winget)
  update/                    # verificação e aplicação de versão nova (release do GitHub)
  tui/                       # interface Bubble Tea
  plain/                     # saída em texto simples
scripts/                     # atualização dos pins e das versões suportadas (usados pelos workflows)
.github/workflows/           # test.yml, release.yml, pins.yml, versoes.yml
.cz.toml                     # configuração do commitizen
```

O pacote `winapi` tem stubs para outros sistemas, então `go vet` e
`go test ./...` também rodam no Linux; o CI roda os testes no Windows e no
Linux.

## Adicionando ou editando um módulo

1. Para só instalar apps, acrescente-os ao catálogo em
   `internal/config/apps.go` (ou crie uma lista nova e um módulo com
   `moduloApps(...)` no arquivo do grupo).
2. Para algo diferente, declare o módulo no arquivo do grupo dele, em
   `internal/modules/`:
   ```go
   var meuModulo = Module{
       ID:    "meu_modulo", // o mesmo do post-install, se fizer a mesma coisa
       Grupo: GrupoDesenvolvimento,
       Title: "Título no menu",
       Desc:  "Descrição de uma linha",
       Deps:  []string{"mise"}, // opcional
       Run: func(c *Ctx) error {
           if err := c.installApp(config.App{ID: "Fornecedor.App", Nome: "App"}); err != nil {
               return err
           }
           c.Log.Successf("Pronto.")
           return nil
       },
   }
   ```
3. Inclua-o em `All()` (`internal/modules/module.go`) entre os módulos do
   mesmo grupo — um teste garante que cada grupo fica junto e na ordem. Se
   um ID mudar de nome, acrescente o antigo a `Renomeados`.

Dentro de `Run`, use os helpers do `Ctx` em vez de `os`/`os/exec`/
`path/filepath` direto, para a saída ir para a TUI/logs, o `--dry-run`
funcionar e os testes golden enxergarem tudo:

- `c.installApp`/`c.installApps` — winget (com o Chocolatey como alternativa)
- `c.Run(...)`/`c.Try(...)`/`c.PowerShell(...)` — comandos e scripts que
  alteram o sistema
- `c.Probe(...)`/`c.Output(...)`/`c.PSOutput(...)` — consultas
  somente-leitura (rodam mesmo no dry-run)
- `c.SetDWord`/`c.SetDWords`/`c.SetString`/`c.DeleteValue`,
  `c.SetUserEnv`, `c.AddToUserPath`/`c.AddToMachinePath`
- `c.WriteFile`/`c.AppendLine`/`c.RemoveLine`/`c.CopyFile`/`c.MkdirAll`
- `c.Get`, `c.Download`, `c.DownloadVerified(url, destino, sha256)`, `c.Temp(nome)`
- `c.Getenv`, `c.Exists`, `c.LookPath`, `c.ReadFile`, `c.Stat`,
  `c.FileExists`, `c.ReadDir`, `c.Glob`, `c.ValueNames` — consultas à
  máquina (no lugar de `os.*`, `exec.LookPath` e `winapi.*`)
- `winpath.Join/Base/Dir` — no lugar do `path/filepath`, para os caminhos
  do Windows saírem iguais no Linux e no Windows
- `c.Log.Infof/Successf/Warnf(...)` — avisos marcam o módulo como "concluído com avisos"

Toda consulta à máquina ou à rede passa pelo `Ctx` porque os **testes
golden** trocam a máquina por uma falsa (`runner.Host`). `golden_test.go`
descreve cenários (um Windows 11 recém-instalado, um já configurado por uma
versão anterior e um Windows 10), roda todos os módulos em dry-run contra
cada um e compara o resultado com
`internal/modules/testdata/golden/<cenário>.txt`. Uma mudança em qualquer
módulo, inclusive no catálogo de apps, aparece como diferença nesses
arquivos. Se a mudança for intencional, regrave e revise o diff antes de
commitar:

```bash
go test ./internal/modules -run TestGolden -update
git diff internal/modules/testdata/golden/
```

O commit fixado pelo `pins.yml` aparece normalizado (`<dotfilesCommit>`),
então os PRs de atualização dos pins não mexem nos golden.

## Detecção de sistema (`internal/sysinfo`)

`sysinfo.Detect` lê a versão no registro
(`HKLM\SOFTWARE\Microsoft\Windows NT\CurrentVersion`), recusa o que não tem
winget (antes do build 19041) e preenche a edição (corrigindo o
`ProductName`, que continua "Windows 10" no Windows 11), a versão
(`DisplayVersion`), o build e a revisão (`UBR`), a arquitetura e, a partir
de `versoes.go`, `NaoTestada` (build mais novo que todos os testados: pede
confirmação) e os avisos de fim de suporte.

### Versões suportadas

`internal/sysinfo/versoes.go` lista as versões do Windows 11 com suporte da
Microsoft nas edições Home/Pro, com o build de cada uma, pelo ciclo de vida
publicado no [endoflife.date](https://endoflife.date/windows) (que segue o
[Windows release health](https://learn.microsoft.com/windows/release-health/windows11-release-information)).

O workflow `versoes.yml` roda o `scripts/atualizar-versoes.sh` toda semana
(ou à mão) e abre um PR na `development` quando a lista muda — uma versão
nova entra, uma fora de suporte sai. A mudança só chega às máquinas na
próxima release do post-windows.

## Desenvolvimento

Requer Go (versão em `go.mod`).

```bash
go test ./...
GOOS=windows GOARCH=amd64 go vet ./...
GOOS=windows GOARCH=amd64 go build -o post-windows.exe ./cmd/post-windows
```

No Windows, `.\post-windows.exe --dry-run` testa a TUI sem Administrador e
sem alterar nada (o `setup.ps1` usa o `post-windows.exe` local, se existir).

## Versionamento e releases

Os commits seguem [Conventional Commits](https://www.conventionalcommits.org/)
(`feat:`, `fix:`, `refactor:`, `feat!:` ...). A cada push na `main`, a action
`.github/workflows/release.yml`:

1. roda as verificações do `test.yml` — no Windows, `go vet`, `go test`,
   `staticcheck`, `govulncheck`, a sintaxe do `setup.ps1` e um `--dry-run`
   do executável; no Linux, `go test -race` e `shellcheck`. O `test.yml`
   também roda em cada PR e push na `development`;
2. usa o [commitizen](https://commitizen-tools.github.io/commitizen/) para
   calcular a próxima versão (`fix` → patch, `feat` → minor, `!`/`BREAKING
   CHANGE` → major), atualizar o `CHANGELOG.md`, criar o commit `bump: ...` e a
   tag `vX.Y.Z` — sem commit `feat`/`fix`/breaking desde a última tag, para aqui;
3. compila o `.exe` (`CGO_ENABLED=0`) para `windows/amd64`, com a versão
   embutida, empacota o `.zip` com o executável + `OVPN\` + `Wallpapers\` +
   `logs\`, gera o atestado de proveniência dos dois e sobe tudo como artefato;
4. publica a release da tag com o `.exe` solto, o `.zip` e os `.sha256`, e
   apaga os artefatos da action.

### Atualização das versões fixadas

Tudo o que é fixado tem quem avise quando sai versão nova — com um PR na
`development`, que chega como notificação do GitHub:

- **Actions e módulos Go** — o Dependabot (`.github/dependabot.yml`), toda
  semana. As actions ficam fixadas pelo SHA do commit, com a tag num
  comentário.
- **Commit dos dotfiles** — o workflow `.github/workflows/pins.yml`, todo dia
  (ou à mão em Actions → Pins → Run workflow). Ele roda
  `scripts/atualizar-pins.sh`, que confere o último commit, atualiza o pin e
  abre o PR `fix: atualiza versões fixadas` com o link de comparação para
  revisar o que mudou.
- **Versões do Windows** — o workflow `versoes.yml` (veja
  [Versões suportadas](#versões-suportadas)).

Os workflows que abrem PR precisam de *Settings → Actions → General → Allow
GitHub Actions to create and approve pull requests*. Os scripts também rodam
localmente: `bash scripts/atualizar-pins.sh`.

## Segurança

- **Versões fixadas.** O repositório de dotfiles (de onde vem a config do Oh
  My Posh, carregada em todo terminal) fica fixado num commit (constante em
  `internal/modules/pins.go`), em vez de seguir a branch principal.
- **Downloads conferidos.** O deploy-dev e o pacote do msphpsql são
  conferidos pelo SHA-256 que o GitHub publica (ou pelo `.sha256` da
  release); o instalador do Composer, pelo SHA-384 publicado pelo projeto;
  as atualizações do próprio post-windows, pelo `.sha256` e pelo digest do
  GitHub. Nenhum download aceita redirecionamento de `https://` para
  `http://`.
- **winget.** Os instaladores do winget são conferidos pelo próprio winget
  (o SHA-256 está no manifesto de cada pacote). Os manifests da comunidade
  têm qualidade variável — veja [Limitações conhecidas](#limitações-conhecidas).
- **O que não é verificado.** O Chocolatey é instalado pelo script oficial
  (`community.chocolatey.org/install.ps1`), executado direto, só quando
  algum app precisa dele (hoje, Nmap e FileZilla); os pacotes dele baixam
  os instaladores dos sites oficiais.
- **Dev Drive.** A unidade fica acessível só para o usuário que roda o
  programa (veja [Dev Drive](#dev-drive)).

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

## Requisitos

- Windows 11 25H2, 26H1 ou 26H2 (ou mais novo, com confirmação; o Windows 10
  2004+ e versões fora de suporte funcionam com aviso) — veja
  [Sistemas suportados](#sistemas-suportados).
- `winget` (App Installer da Microsoft Store atualizado).
- Conta com direitos de Administrador (o programa se reabre pelo UAC) —
  exceto em `--dry-run`.
- Terminal interativo para a TUI (sem ele, o modo texto é usado).

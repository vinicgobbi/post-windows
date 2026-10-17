#Requires -Version 5.1
<#
    Bootstrap do post-windows: baixa o executável da release no GitHub
    (verificando o SHA-256) e executa como Administrador, apontando OVPN\ e
    logs\ para esta pasta.

      powershell -ExecutionPolicy Bypass -File .\setup.ps1              # última release
      $env:POST_WINDOWS_VERSION = 'v1.2.0'; .\setup.ps1                 # versão específica
      powershell -ExecutionPolicy Bypass -File .\setup.ps1 --dry-run    # flags são repassadas

    Se existir um .\post-windows.exe compilado localmente
    (go build -o post-windows.exe .\cmd\post-windows), ele é usado no lugar
    do download.
#>

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'   # o Invoke-WebRequest fica muito lento com a barra
$Repo = 'vinicgobbi/post-windows'
$ScriptDir = $PSScriptRoot

# Eleva aqui (e não no executável) para tudo rodar nesta mesma janela.
$principal = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator) -and ($args -notcontains '--dry-run')) {
    Write-Host '[!] É preciso rodar como Administrador. Reabrindo com o UAC...' -ForegroundColor Yellow
    $q = { param($s) "'" + ($s -replace "'", "''") + "'" }
    $repassar = ($args | ForEach-Object { & $q $_ }) -join ' '
    $versao = if ($env:POST_WINDOWS_VERSION) { "`$env:POST_WINDOWS_VERSION = $(& $q $env:POST_WINDOWS_VERSION); " } else { '' }
    $comando = "$versao& $(& $q $PSCommandPath) $repassar; Read-Host 'Pressione Enter para fechar'"
    $psi = New-Object System.Diagnostics.ProcessStartInfo
    $psi.FileName = (Get-Process -Id $PID).Path
    # -EncodedCommand: o -Command do powershell.exe trata aspas de um jeito
    # próprio, que quebra caminhos e argumentos com espaço.
    $psi.Arguments = '-NoProfile -ExecutionPolicy Bypass -EncodedCommand ' + [Convert]::ToBase64String([Text.Encoding]::Unicode.GetBytes($comando))
    $psi.Verb = 'runas'
    # Sem UseShellExecute = $true o "runas" é ignorado em silêncio no .NET
    # moderno (PowerShell 7), reabrindo sem elevar.
    $psi.UseShellExecute = $true
    try { [System.Diagnostics.Process]::Start($psi) | Out-Null } catch { Write-Host '[-] Elevação cancelada.' -ForegroundColor Red; exit 1 }
    exit 0
}

# Só há build para amd64.
if ($env:PROCESSOR_ARCHITECTURE -ne 'AMD64') {
    Write-Host "[-] Arquitetura não suportada: $env:PROCESSOR_ARCHITECTURE (só amd64/x64)." -ForegroundColor Red
    exit 1
}

$bin = Join-Path $ScriptDir 'post-windows.exe'
if (-not (Test-Path $bin)) {
    [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12
    $asset = 'post-windows-windows-amd64.exe'
    $url = if ($env:POST_WINDOWS_VERSION) {
        "https://github.com/$Repo/releases/download/$env:POST_WINDOWS_VERSION/$asset"
    } else {
        "https://github.com/$Repo/releases/latest/download/$asset"
    }

    $tmp = Join-Path $env:TEMP ('post-windows-' + [guid]::NewGuid().ToString('N'))
    New-Item -ItemType Directory -Path $tmp | Out-Null
    $bin = Join-Path $tmp $asset
    Write-Host "[*] Baixando $url" -ForegroundColor Yellow
    Invoke-WebRequest -Uri $url -OutFile $bin -UseBasicParsing
    Invoke-WebRequest -Uri "$url.sha256" -OutFile "$bin.sha256" -UseBasicParsing

    # Formato do sha256sum: "<hash>  <arquivo>".
    $esperado = ((Get-Content "$bin.sha256" -Raw).Trim() -split '\s+')[0].ToLower()
    $real = (Get-FileHash -Path $bin -Algorithm SHA256).Hash.ToLower()
    if ($real -ne $esperado) {
        Write-Host '[-] Checksum SHA-256 não confere; abortando.' -ForegroundColor Red
        Remove-Item $tmp -Recurse -Force
        exit 1
    }
}

& $bin --ovpn-dir (Join-Path $ScriptDir 'OVPN') --log-dir (Join-Path $ScriptDir 'logs') @args
$codigo = $LASTEXITCODE
if ($tmp) { Remove-Item $tmp -Recurse -Force -ErrorAction SilentlyContinue }
exit $codigo

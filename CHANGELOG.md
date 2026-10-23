## v1.2.0 (2026-10-23)

### Feat

- instala ferramentas de terminal, Ruby/Jekyll e deploy-dev
- pastas de cache e pacotes de dev fora da indexação
- Dev Drive e pasta de projetos fora da indexação do Windows
- Dev Drive acessível só para o usuário que roda o programa
- troca o Microsoft 365 pelo LibreOffice e OnlyOffice

### Fix

- mantém o Family Safety funcionando com as otimizações

## v1.1.0 (2026-10-17)

### Feat

- adiciona o Firefox (pt-BR) aos navegadores
- setup.ps1 baixa e executa o executável da release
- executável com flags, elevação via UAC e modo texto
- interface TUI
- customização, desempenho, debloat e limpeza do Windows
- Dev Drive para projetos e caches de pacotes
- PowerShell 7 e Oh My Posh com a config dos dotfiles
- PHP com php.ini fixo fora da pasta do pacote
- Microsoft 365 Apps via Office Deployment Tool
- módulos de apps, Windows Update, VirtualBox, Tailscale e OpenVPN
- base dos módulos e instalação via winget/Chocolatey
- catálogo de apps do winget
- engine de dependências e execução dos módulos
- detecção da edição, versão e build do Windows
- runner para comandos, PowerShell, registro e downloads
- logs da execução e saída em texto simples
- pacote winapi para as chamadas exclusivas do Windows
- Enhance performance optimizations by disabling telemetry services, restricting background apps, and ensuring SSD TRIM is active
- Add functionality to create and pin Projects folder to Quick Access, enhance error handling in setup script
- Enhance VSCode installation with context menu options and update installation script for development tools
- Improve installation handling for Spotify and WhatsApp, resolve winget alias issue, and update PHP version to 8.5
- Update game launcher installation messages for clarity and accuracy
- Add Composer installation function and enhance performance optimizations for hibernation settings
- Add support for user-scoped installations of Spotify and WhatsApp to handle elevation issues
- Refactor installation scripts for various applications to improve readability and maintainability
- Enhance bloatware removal by adding more apps to the removal list and improving package removal logic
- Ensure elevated script execution by explicitly setting UseShellExecute to true
- Update visual customization to disable Start menu recommendations and improve search box settings
- Update visual customization to center taskbar icons and hide search bar
- Enhance bloatware removal by adding Xbox family apps and disabling web search and telemetry tasks
- Enhance PowerShell profile setup with personal dotfiles configuration and improved cloning logic
- Add performance optimizations and bloatware removal modules for Windows setup
- Add utility functions and modules for Windows setup automation

### Fix

- instala o Firefox em inglês (Mozilla.Firefox)

### Refactor

- remove a implementação em PowerShell

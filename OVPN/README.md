# Perfis OpenVPN

Coloque aqui dentro todos os arquivos `.ovpn` que devem ser importados pelo
módulo `ovpn` durante o setup.

O módulo instala o OpenVPN GUI e copia todo `*.ovpn` presente nesta pasta
para a pasta de perfis monitorada por ele (`%ProgramFiles%\OpenVPN\config`).
Cada arquivo vira um perfil separado no ícone da bandeja.

Os arquivos `.ovpn` (e certificados/chaves que venham com eles) não são
versionados — só este README fica no repositório (veja `.gitignore`).

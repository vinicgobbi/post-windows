package modules

import (
	"strings"

	"github.com/vinicgobbi/post-windows/internal/config"
	"github.com/vinicgobbi/post-windows/internal/winpath"
)

var tailscale = Module{
	ID:    "tailscale",
	Grupo: GrupoRede,
	Title: "Tailscale",
	Desc:  "Tailscale (VPN mesh) com o cliente gráfico nativo do Windows, na bandeja (o equivalente ao Trayscale)",
	Run: func(c *Ctx) error {
		// O cliente Windows já vem com ícone de bandeja e app gráfico —
		// diferente do Linux, não precisa de um "Trayscale" separado.
		if err := c.installApp(config.App{ID: "Tailscale.Tailscale", Nome: "Tailscale"}); err != nil {
			return err
		}
		c.Log.Infof("Abra o Tailscale pelo ícone da bandeja e faça login para conectar este PC à sua tailnet (o login é interativo).")
		return nil
	},
}

var ovpn = Module{
	ID:    "ovpn",
	Grupo: GrupoRede,
	Title: "Importar perfis OpenVPN",
	Desc:  "Instala o OpenVPN GUI e copia os .ovpn de .\\OVPN para a pasta de perfis monitorada por ele",
	Run: func(c *Ctx) error {
		if err := c.installApp(config.App{ID: "OpenVPNTechnologies.OpenVPN", Nome: "OpenVPN GUI"}); err != nil {
			return err
		}
		arquivos, _ := c.Glob(winpath.Join(c.OVPNDir, "*.ovpn"))
		if len(arquivos) == 0 {
			c.Log.Warnf("Nenhum arquivo .ovpn encontrado em %s — pulando importação.", c.OVPNDir)
			return nil
		}
		// O OpenVPN GUI lista cada .ovpn desta pasta como um perfil; DNS e
		// domínio do dhcp-option já são aplicados pelo próprio cliente.
		destino := winpath.Join(c.Getenv("ProgramFiles"), "OpenVPN", "config")
		for _, a := range arquivos {
			if err := c.CopyFile(a, winpath.Join(destino, winpath.Base(a))); err != nil {
				return err
			}
			c.Log.Successf("Perfil '%s' importado.", strings.TrimSuffix(winpath.Base(a), ".ovpn"))
		}
		c.Log.Infof("Abra o OpenVPN GUI (ícone na bandeja) para conectar — a autenticação é manual.")
		return nil
	},
}

var virtualbox = Module{
	ID:    "virtualbox",
	Grupo: GrupoRede,
	Title: "VirtualBox",
	Desc:  "Gerenciador de máquinas virtuais VirtualBox (o equivalente ao virt-manager); funciona em qualquer edição, inclusive Home",
	Run: func(c *Ctx) error {
		if out, err := c.PSOutput(`(Get-CimInstance Win32_Processor | Select-Object -First 1).VirtualizationFirmwareEnabled`); err == nil && strings.EqualFold(out, "False") {
			c.Log.Warnf("Virtualização por hardware (VT-x/AMD-V) não está habilitada na BIOS/UEFI — o VirtualBox pode não funcionar direito.")
		}
		if out, err := c.PSOutput(`(Get-WindowsOptionalFeature -Online -FeatureName Microsoft-Hyper-V-All -ErrorAction SilentlyContinue).State`); err == nil && out == "Enabled" {
			c.Log.Warnf("O Hyper-V está habilitado: o VirtualBox roda em cima dele (via API do Windows), o que funciona mas pode reduzir a performance.")
		}
		return c.installApp(config.App{ID: "Oracle.VirtualBox", Nome: "VirtualBox"})
	},
}

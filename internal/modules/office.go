package modules

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/vinicgobbi/post-windows/internal/config"
)

var office = Module{
	ID:    "office",
	Title: "Instalar Microsoft 365 Apps",
	Desc:  "Word/Excel/PowerPoint/Outlook via Office Deployment Tool, no lugar do LibreOffice/OnlyOffice",
	Run: func(c *Ctx) error {
		c.Log.Infof("Instalando o Microsoft 365 Apps (substitui LibreOffice/OnlyOffice)...")
		if err := c.installApp(config.App{ID: "Microsoft.OfficeDeploymentTool", Nome: "Office Deployment Tool"}); err != nil {
			return err
		}

		odtDir := filepath.Join(os.Getenv("ProgramFiles"), "OfficeDeploymentTool")
		setup := filepath.Join(odtDir, "setup.exe")
		if _, err := os.Stat(setup); err != nil && !c.DryRun {
			c.Log.Warnf("setup.exe do ODT não encontrado em %s; pulando a instalação do Office.", odtDir)
			return nil
		}

		xml := filepath.Join(odtDir, "configuration.xml")
		if err := c.WriteFile(xml, officeConfig(c.OfficeProduct)); err != nil {
			return err
		}
		c.Log.Infof("Baixando e instalando o Office via ODT (pode demorar alguns minutos)...")
		if err := c.Run(setup, "/configure", xml); err != nil {
			if c.Ctx.Err() != nil {
				return err
			}
			c.Log.Warnf("O ODT terminou com erro (%v); confira o log do Office em %%TEMP%%.", err)
			return nil
		}
		c.Log.Successf("Microsoft 365 Apps instalado. Abra qualquer app do Office e faça login com sua conta Microsoft 365 para ativar.")
		return nil
	},
}

func officeConfig(produto string) string {
	if produto == "" {
		produto = config.OfficeProductID
	}
	return fmt.Sprintf(`<Configuration>
  <Add OfficeClientEdition="64" Channel="Current">
    <Product ID="%s">
      <Language ID="MatchOS" />
    </Product>
  </Add>
  <Updates Enabled="TRUE" />
  <Display Level="Full" AcceptEULA="TRUE" />
</Configuration>
`, produto)
}

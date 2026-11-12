package modules

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/vinicgobbi/post-windows/internal/winpath"
)

var papelParede = Module{
	ID:    "papel_parede",
	Grupo: GrupoWindows,
	Title: "Papel de parede",
	Desc:  "Copia o papel de parede escolhido para %LOCALAPPDATA%\\post-windows\\Wallpapers e aplica no Windows (modo Preencher)",
	Run: func(c *Ctx) error {
		if c.Wallpaper == "" {
			c.Log.Infof("Nenhum papel de parede escolhido: mantendo o atual.")
			return nil
		}
		// Fica numa pasta própria (e não em Imagens, que pode estar no
		// OneDrive): o Windows lê a imagem de lá a cada logon.
		destino := winpath.Join(c.Getenv("LOCALAPPDATA"), "post-windows", "Wallpapers", winpath.Base(c.Wallpaper))
		c.Log.Infof("Aplicando o papel de parede '%s'...", winpath.Base(c.Wallpaper))
		err := step(
			func() error { return c.CopyFile(c.Wallpaper, destino) },
			// Preencher (WallpaperStyle 10): cobre a tela sem distorcer,
			// cortando as sobras — o "zoom" do GNOME.
			func() error { return c.SetString(`HKCU\Control Panel\Desktop`, "WallpaperStyle", "10") },
			func() error { return c.SetString(`HKCU\Control Panel\Desktop`, "TileWallpaper", "0") },
			func() error { return c.SetWallpaper(destino) },
		)
		if err != nil {
			return err
		}
		c.Log.Successf("Papel de parede aplicado: %s", destino)
		return nil
	},
}

// extensoesWallpaper são os formatos que o Windows aplica como papel de
// parede sem extensão da Store (o WebP precisa da "Extensões de Imagem da
// Web").
var extensoesWallpaper = []string{".jpg", ".jpeg", ".png", ".bmp"}

// ListWallpapers devolve as imagens da pasta de papéis de parede, em ordem
// alfabética. Roda antes da interface, na máquina real.
func ListWallpapers(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if !e.IsDir() && slices.Contains(extensoesWallpaper, ext) {
			out = append(out, filepath.Join(dir, e.Name()))
		}
	}
	return out
}

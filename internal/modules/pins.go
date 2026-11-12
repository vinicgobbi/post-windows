package modules

// Código dos repositórios pessoais que este programa usa fica fixado numa
// versão conhecida: uma conta comprometida ou um push errado na branch
// principal não chega sozinho às próximas instalações. Para atualizar, troque
// o commit aqui (o scripts/atualizar-pins.sh faz isso, e o workflow pins.yml
// abre o PR).
const (
	// Os dotfiles trazem a config do Oh My Posh (pasta oh-my-posh/: tema,
	// aliases, editor), carregada em todo terminal do PowerShell.
	dotfilesRepo   = "https://github.com/vinicgobbi/Dotfiles.git"
	dotfilesCommit = "1e9a97699b70582f87e768f1fe6aa1dde9b58fd6"
)

// clonarFixado clona repo em dir e posiciona no commit ref.
func clonarFixado(c *Ctx, repo, ref, dir string) error {
	return step(
		func() error { return c.Run("git", "clone", "--quiet", repo, dir) },
		func() error {
			return c.Run("git", "-C", dir, "-c", "advice.detachedHead=false", "checkout", "--quiet", ref)
		},
	)
}

// Package winpath manipula caminhos no formato do Windows (C:\Users\x) do
// mesmo jeito em qualquer sistema. O path/filepath segue o sistema em que o
// programa roda: no Linux, filepath.Join(`C:\Users\x`, "y") daria
// `C:\Users\x/y` e filepath.Base não entenderia a barra invertida. Os
// módulos usam este pacote para os testes golden do dry-run
// (internal/modules/testdata/golden) darem o mesmo resultado no Linux e no
// Windows.
package winpath

import (
	"path"
	"strings"
)

// toSlash e fromSlash trocam as barras para usar o pacote path, que trata
// "/" igual em qualquer sistema.
func toSlash(p string) string   { return strings.ReplaceAll(p, `\`, "/") }
func fromSlash(p string) string { return strings.ReplaceAll(p, "/", `\`) }

// raiz completa a unidade sozinha ("C:") com a barra ("C:\"), que é a raiz
// dela (como o filepath faz no Windows).
func raiz(p string) string {
	if len(p) == 2 && p[1] == ':' {
		return p + `\`
	}
	return p
}

// Join junta os elementos com "\" e limpa o resultado (como o filepath.Join
// do Windows).
func Join(elem ...string) string {
	partes := make([]string, 0, len(elem))
	for _, e := range elem {
		if e != "" {
			partes = append(partes, toSlash(e))
		}
	}
	if len(partes) == 0 {
		return ""
	}
	return raiz(fromSlash(path.Join(partes...)))
}

// Clean equivale ao filepath.Clean do Windows.
func Clean(p string) string {
	if p == "" {
		return "."
	}
	return raiz(fromSlash(path.Clean(toSlash(p))))
}

// Base devolve o último elemento do caminho.
func Base(p string) string {
	return fromSlash(path.Base(toSlash(strings.TrimRight(p, `\/`))))
}

// Dir devolve o caminho sem o último elemento.
func Dir(p string) string {
	return raiz(fromSlash(path.Dir(toSlash(p))))
}

// Ext devolve a extensão do último elemento (com o ponto).
func Ext(p string) string { return path.Ext(Base(p)) }

// Rel devolve alvo relativo a base, se alvo estiver dentro de base (sem
// diferenciar maiúsculas, como o sistema de arquivos do Windows).
func Rel(base, alvo string) (string, bool) {
	b := strings.TrimRight(Clean(base), `\`) + `\`
	a := Clean(alvo)
	if len(a) < len(b) || !strings.EqualFold(a[:len(b)], b) {
		return "", false
	}
	return a[len(b):], true
}

// Igual compara dois caminhos sem diferenciar maiúsculas nem a barra final.
func Igual(a, b string) bool {
	return strings.EqualFold(strings.TrimRight(Clean(a), `\`), strings.TrimRight(Clean(b), `\`))
}

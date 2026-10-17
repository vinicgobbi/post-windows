// Package winapi isola tudo que só existe no Windows (registro, UAC, PATH do
// sistema, broadcast de configurações, encerramento de árvore de processos).
//
// Em outros sistemas as funções devolvem ErrSoWindows (ou não fazem nada),
// para o resto do programa — e os testes — compilarem e rodarem no Linux.
package winapi

import (
	"errors"
	"fmt"
	"strings"
)

var ErrSoWindows = errors.New("disponível só no Windows")

// Raízes de registro aceitas em SplitKey.
var raizes = map[string]string{
	"HKCU": "HKCU", "HKEY_CURRENT_USER": "HKCU",
	"HKLM": "HKLM", "HKEY_LOCAL_MACHINE": "HKLM",
}

// SplitKey separa "HKCU\Software\X" (ou "HKCU:\Software\X", como no
// PowerShell) em raiz ("HKCU"/"HKLM") e subchave.
func SplitKey(key string) (root, sub string, err error) {
	r, s, _ := strings.Cut(key, `\`)
	r = strings.TrimSuffix(strings.ToUpper(r), ":")
	root, ok := raizes[r]
	if !ok || s == "" {
		return "", "", fmt.Errorf("chave de registro inválida: %q", key)
	}
	return root, s, nil
}

// QuoteArg escapa um argumento de linha de comando seguindo as regras do
// CommandLineToArgvW (as mesmas do syscall.EscapeArg do Go no Windows), para
// montar linhas de comando em texto (ShellExecute, tarefa agendada).
func QuoteArg(s string) string {
	if s == "" {
		return `""`
	}
	if !strings.ContainsAny(s, " \t\"") {
		return s
	}
	var b strings.Builder
	b.WriteByte('"')
	barras := 0
	for _, c := range s {
		switch c {
		case '\\':
			barras++
			continue
		case '"':
			b.WriteString(strings.Repeat(`\`, barras*2+1))
		default:
			b.WriteString(strings.Repeat(`\`, barras))
		}
		barras = 0
		b.WriteRune(c)
	}
	b.WriteString(strings.Repeat(`\`, barras*2))
	b.WriteByte('"')
	return b.String()
}

// JoinArgs junta argumentos numa linha de comando (ver QuoteArg).
func JoinArgs(args []string) string {
	q := make([]string, len(args))
	for i, a := range args {
		q[i] = QuoteArg(a)
	}
	return strings.Join(q, " ")
}

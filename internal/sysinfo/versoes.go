// Versões suportadas — mantido pelo scripts/atualizar-versoes.sh, que o
// workflow versoes.yml roda toda semana, abrindo um PR quando algo muda.
// Pode ser editado à mão, mas o script regrava o arquivo inteiro.

package sysinfo

// windowsSuportadas são as versões do Windows 11 com suporte da Microsoft
// nas edições Home/Pro, da lançada há mais tempo para a mais recente. Build
// é o número do build (o "26200" de 10.0.26200).
var windowsSuportadas = []versaoWindows{
	{Versao: "25H2", Build: 26200},
	{Versao: "26H1", Build: 28000},
	{Versao: "26H2", Build: 26300},
}

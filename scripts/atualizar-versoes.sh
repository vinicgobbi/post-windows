#!/usr/bin/env bash
# Regrava internal/sysinfo/versoes.go com as versões do Windows suportadas, a
# partir do ciclo de vida publicado no endoflife.date (que segue o
# learn.microsoft.com/windows/release-health). Usado pelo workflow
# .github/workflows/versoes.yml (que abre um PR com o resultado), mas também
# roda à mão a partir da raiz do repositório. Requer curl e jq.
#
#   - Windows 11 nas edições Home/Pro (as "W" do endoflife.date: Home, Pro,
#     Pro Education e Pro for Workstations), já lançadas e com fim de
#     suporte ainda não alcançado. O build é o do "latest" de cada versão
#     (10.0.26200 -> 26200).
#
# O resumo das mudanças (em Markdown, para o corpo do PR) vai para o arquivo
# em $VERSOES_RESUMO (padrão: versoes-resumo.md). $HOJE (AAAA-MM-DD) troca a
# data de referência, para testar.
set -euo pipefail

ARQ="internal/sysinfo/versoes.go"
RESUMO="${VERSOES_RESUMO:-versoes-resumo.md}"
HOJE="${HOJE:-$(date -u +%F)}"
API="https://endoflife.date/api/v1/products/windows/"

: >"$RESUMO"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

curl -fsSL --retry 2 --proto '=https' "$API" |
    jq -r --arg hoje "$HOJE" '.result.releases[]
        | select(.name | test("^11-[0-9]{2}h[0-9]-w$"))
        | select(.releaseDate <= $hoje and (.eolFrom == null or .eolFrom > $hoje))
        | [.releaseDate, (.name | capture("^11-(?<v>[0-9]{2}h[0-9])-w$").v | ascii_upcase),
           (.latest.name | split(".") | .[2])]
        | @tsv' | sort >"$TMP/windows"

linhas=""
while IFS=$'\t' read -r _ versao build; do
    if ! [[ "$build" =~ ^[0-9]{5}$ ]]; then
        echo "erro: build inválido para o Windows 11 $versao: '$build'" >&2
        exit 1
    fi
    linhas+=$'\t'"{Versao: \"$versao\", Build: $build},"$'\n'
done <"$TMP/windows"

# Uma resposta vazia (fonte fora do ar, formato mudado) não pode virar um PR
# que tira o suporte a tudo.
if [[ -z "$linhas" ]]; then
    echo "erro: lista de versões vazia; nada alterado" >&2
    exit 1
fi

cat >"$TMP/versoes.go" <<EOF
// Versões suportadas — mantido pelo scripts/atualizar-versoes.sh, que o
// workflow versoes.yml roda toda semana, abrindo um PR quando algo muda.
// Pode ser editado à mão, mas o script regrava o arquivo inteiro.

package sysinfo

// windowsSuportadas são as versões do Windows 11 com suporte da Microsoft
// nas edições Home/Pro, da lançada há mais tempo para a mais recente. Build
// é o número do build (o "26200" de 10.0.26200).
var windowsSuportadas = []versaoWindows{
${linhas}}
EOF

if cmp -s "$TMP/versoes.go" "$ARQ"; then
    echo "Versões suportadas sem mudança."
    exit 0
fi

# Resumo: as entradas que saem e as que entram.
entradas() { grep -oE '\{Versao: [^}]*\}' "$1" | sed -E 's/[{}]//g' || true; }
# (o diff sai com 1 quando há diferença, o que é o caso aqui). As crases do
# sed são Markdown literal para o corpo do PR, não substituição de comando.
# shellcheck disable=SC2016
{ diff <(entradas "$ARQ" 2>/dev/null) <(entradas "$TMP/versoes.go") || true; } | sed -nE \
    -e 's/^< (.*)/- **sai:** `\1`/p' \
    -e 's/^> (.*)/- **entra:** `\1`/p' | tee -a "$RESUMO"
cp "$TMP/versoes.go" "$ARQ"

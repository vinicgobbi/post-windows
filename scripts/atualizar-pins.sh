#!/usr/bin/env bash
# Confere se há versão nova de cada código pessoal fixado no post-windows e,
# se houver, atualiza o pin no lugar. Usado pelo workflow
# .github/workflows/pins.yml (que abre um PR com o resultado), mas também
# roda à mão a partir da raiz do repositório.
#
# Pins conferidos:
#   - internal/modules/pins.go: pares <nome>Repo/<nome>Commit (último
#     commit da branch principal)
#
# O resumo das mudanças (em Markdown, para o corpo do PR) vai para o arquivo
# em $PINS_RESUMO (padrão: pins-resumo.md).
set -euo pipefail

PINS="internal/modules/pins.go"
RESUMO="${PINS_RESUMO:-pins-resumo.md}"

: >"$RESUMO"

anotar() { echo "- $*" | tee -a "$RESUMO"; }

# URL web do repositório (sem .git), para os links de comparação.
web() { echo "${1%.git}"; }

ultimo_commit() { git ls-remote "$1" HEAD | cut -f1; }

# ---- Commits fixados em pins.go ----
while read -r nome atual; do
    repo="$(sed -nE "s/^\s*${nome}Repo\s*=\s*\"([^\"]+)\".*/\1/p" "$PINS")"
    if [[ -z "$repo" ]]; then
        echo "aviso: ${nome}Repo não encontrado em $PINS" >&2
        continue
    fi
    novo="$(ultimo_commit "$repo")"
    if [[ -n "$novo" && "$novo" != "$atual" ]]; then
        sed -i "s/$atual/$novo/" "$PINS"
        anotar "**${nome}**: [\`${atual:0:7}...${novo:0:7}\`]($(web "$repo")/compare/$atual...$novo)"
    fi
done < <(sed -nE 's/^\s*(\w+)Commit\s*=\s*"([0-9a-f]{40})".*/\1 \2/p' "$PINS")

if [[ ! -s "$RESUMO" ]]; then
    echo "Todos os pins já estão na versão mais recente."
fi

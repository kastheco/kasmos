#!/usr/bin/env bash
set -euo pipefail

MAP="${1:-docs/docs-drift-map.yml}"
BASE_REF="${BASE_REF:-$(git describe --tags --abbrev=0 --match 'v*' 2>/dev/null || echo main)}"

if ! git rev-parse --verify "$BASE_REF^{commit}" >/dev/null 2>&1; then
	if git rev-parse --verify "origin/$BASE_REF^{commit}" >/dev/null 2>&1; then
		BASE_REF="origin/$BASE_REF"
	fi
fi

join_by_comma() {
	local IFS=,
	printf '%s' "$*"
}

changed_paths() {
	git diff --name-only "$BASE_REF"...HEAD -- "$@" 2>/dev/null || true
}

echo '{"drift": ['
first=true
yq e -o=json -I=0 '.[]' "$MAP" | while IFS= read -r entry; do
	mapfile -t code < <(printf '%s\n' "$entry" | yq -r '.code_paths[]')
	mapfile -t docs < <(printf '%s\n' "$entry" | yq -r '.docs_paths[]')

	mapfile -t changed_code_paths < <(changed_paths "${code[@]}")
	if [ "${#changed_code_paths[@]}" -eq 0 ]; then
		continue
	fi

	mapfile -t changed_doc_paths < <(changed_paths "${docs[@]}")
	if [ "${#changed_doc_paths[@]}" -eq 0 ]; then
		$first || echo ','
		first=false
		jq -nc \
			--arg code "$(join_by_comma "${changed_code_paths[@]}")" \
			--argjson docs "$(printf '%s\n' "${docs[@]}" | jq -R . | jq -s .)" \
			'{code_changed: $code, docs_not_changed: $docs}'
	fi
done
echo ']}'

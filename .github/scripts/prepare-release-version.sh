#!/usr/bin/env bash

set -euo pipefail

if [ "${GITHUB_REF_NAME:-}" != "main" ]; then
	echo "Release 只能从 main 分支发布。" >&2
	exit 1
fi

tag_exists() {
	local tag="$1"
	local status
	if git ls-remote --exit-code --tags origin "refs/tags/${tag}" >/dev/null 2>&1; then
		return 0
	else
		status=$?
	fi
	if [ "${status}" -eq 2 ]; then
		return 1
	fi
	echo "读取远端 Release 标签失败：${tag}" >&2
	exit "${status}"
}

core_makefile="package/openwrt2mqtt/Makefile"
luci_makefile="package/luci-app-openwrt2mqtt/Makefile"
core_version="$(sed -n 's/^PKG_VERSION:=//p' "${core_makefile}")"
luci_version="$(sed -n 's/^PKG_VERSION:=//p' "${luci_makefile}")"
core_release="$(sed -n 's/^PKG_RELEASE:=//p' "${core_makefile}")"
luci_release="$(sed -n 's/^PKG_RELEASE:=//p' "${luci_makefile}")"

if [ "${core_version}" != "${luci_version}" ] || [ "${core_release}" != "${luci_release}" ]; then
	echo "核心包和 LuCI 包版本不一致。" >&2
	exit 1
fi
if [[ ! "${core_version}" =~ ^([0-9]+)\.([0-9]+)\.([0-9]+)$ ]]; then
	echo "PKG_VERSION 必须使用 X.Y.Z 格式。" >&2
	exit 1
fi
major="${BASH_REMATCH[1]}"
minor="${BASH_REMATCH[2]}"
patch="${BASH_REMATCH[3]}"
if [[ ! "${core_release}" =~ ^[1-9][0-9]*$ ]]; then
	echo "PKG_RELEASE 必须为正整数。" >&2
	exit 1
fi
version="${core_version}"
tag="v${version}"
base_sha="$(git rev-parse HEAD)"
version_changed=0
if tag_exists "${tag}"; then
	while :; do
		patch="$((patch + 1))"
		version="${major}.${minor}.${patch}"
		tag="v${version}"
		if ! tag_exists "${tag}"; then
			break
		fi
	done

	sed -i -E "s/^PKG_VERSION:=.*/PKG_VERSION:=${version}/; s/^PKG_RELEASE:=.*/PKG_RELEASE:=1/" \
		"${core_makefile}" "${luci_makefile}"
	git config user.name "github-actions[bot]"
	git config user.email "41898282+github-actions[bot]@users.noreply.github.com"
	git add "${core_makefile}" "${luci_makefile}"
	git commit -m "chore: prepare release v${version}"
	version_changed=1
fi

git fetch origin main
if [ "${base_sha}" != "$(git rev-parse origin/main)" ]; then
	echo "main 分支已更新，请重新触发 Release 构建。" >&2
	exit 1
fi
if [ "${version_changed}" -eq 1 ]; then
	git push origin HEAD:main
fi

if [ -n "${GITHUB_OUTPUT:-}" ]; then
	{
		echo "version=${version}"
		echo "source_sha=$(git rev-parse HEAD)"
	} >> "${GITHUB_OUTPUT}"
else
	printf 'version=%s\nsource_sha=%s\n' "${version}" "$(git rev-parse HEAD)"
fi

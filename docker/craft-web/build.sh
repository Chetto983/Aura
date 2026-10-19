#!/bin/sh
# Builds Aura's web build of one Craft app into <out>/<app>
# (docs/superpowers/specs/2026-10-19-craft-web-apps-design.md):
#
#   build.sh <app> <tag> <commit> <out>
#
# The release tag's source, checked against its commit, plus <app>.patch beside this script: the
# `?host=parent` bridge the cockpit opens and saves files through, and the ArtCraft marks the
# brand terms (docs/brand/LICENSE-brand.txt upstream) keep out of a modified build. The tools come
# from PATH (trunk, wasm-bindgen, wasm-opt, pinned by the Dockerfile), never downloaded here.
set -eu

app=$1 tag=$2 commit=$3 out=$4
here=$(cd "$(dirname "$0")" && pwd)
work=$(mktemp -d)
src=$work/$app

git clone --quiet --depth 1 --branch "$tag" "https://github.com/storytold/$app" "$src"
if [ "$(git -C "$src" rev-parse HEAD)" != "$commit" ]; then
    echo "craft-web: $app $tag is $(git -C "$src" rev-parse HEAD), not $commit" >&2
    exit 1
fi
git -C "$src" apply "$here/$app.patch"
# A translated row naming ArtCraft goes: the patch rewords or drops the English it translated.
if [ -d "$src/crates/ui-egui/src/i18n" ]; then
    find "$src/crates/ui-egui/src/i18n" -name '*.tsv' -exec sed -i '/artcraft/Id' {} +
fi

export CARGO_TARGET_DIR="$work/target"
if [ "$app" = filmcraft ]; then
    (cd "$src" && cargo xtask web)
    dist=$CARGO_TARGET_DIR/web/dist
else
    (cd "$src/apps/$app-web" && trunk build --release --offline)
    dist=$src/dist/web
fi

mkdir -p "$out/$app"
cp -R "$dist"/. "$out/$app/"
cp "$src/LICENSE-MIT" "$src/LICENSE-APACHE" "$out/$app/"
# Nothing in front of aura compresses: aura sends the .gz to a browser that accepts it
# (internal/staticapp).
gzip -9 -k "$out/$app"/*.wasm "$out/$app"/*.js
rm -rf "$work"

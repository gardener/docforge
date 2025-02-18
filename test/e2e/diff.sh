#!/usr/bin/env bash

# SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -e

docforge_repo_path="$(readlink -f "$(dirname "${0}")/../..")"
hugo=${docforge_repo_path}/test/e2e/hugo
PR_hugo=${docforge_repo_path}/test/e2e/hugoPR
docforge_bin="${docforge_repo_path}/bin/docforge"

echo Docforge repo: "$docforge_repo_path"

echo "Clean old build if done"
rm -rf "$hugo"
rm -rf "$PR_hugo"

buildWebsite() {
  LOCAL_BUILD=1 "${docforge_repo_path}/.ci/build" >/dev/null 2>&1
  DOCFORGE_CONFIG="${docforge_repo_path}/test/e2e/docforge_config.yaml" "$docforge_bin"
}

echo "Building current docforge"
buildWebsite
mv "$hugo" "$PR_hugo"

echo "Building master docforge"
pushd "$docforge_repo_path"
cp "VERSION" "VERSION_PR" 
git checkout -- "VERSION"
current_branch=$(git branch --show-current)
git checkout master
popd
buildWebsite

echo "-------------------------------"
echo "Diff results"
rm -rf "${hugo}/content/__resources" 
rm -rf "${PR_hugo}/content/__resources" 
diff -r "$hugo" "$PR_hugo"
rm -rf "${hugo}"
rm -rf "${PR_hugo}"

pushd "$docforge_repo_path"
if [[ -n "$current_branch" ]]; then
    git checkout "$current_branch"
else
    echo "current_branch is empty"
fi
mv "VERSION_PR" "VERSION"

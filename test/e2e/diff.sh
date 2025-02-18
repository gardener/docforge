#!/usr/bin/env bash

# SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -e

getGitHubToken() {
  # Check if gardener-ci is available (in local setup)
  command -v gardener-ci >/dev/null && gardenci="true" || gardenci=""
  if [[ $gardenci == "true" ]]; then
    # Get a (round-robin) random technical GitHub user credentials
    technicalUser=$(gardener-ci config model_element --cfg-type github --cfg-name "${1}" --key credentials | sed -e "s/^GithubCredentials //" -e "s/'/\"/g")
    if [[ -n "${technicalUser}" ]]; then
      # get auth token and strip lead/trail quotes
      authToken="$(jq -r '.authToken' <<< "$technicalUser")"
      echo "${authToken}"
    fi
  fi
}

if [[ $(uname) == 'Darwin' ]]; then
  READLINK_BIN="greadlink"
else
  READLINK_BIN="readlink"
fi

docforge_repo_path="$(${READLINK_BIN} -f "$(dirname "${0}")/../..")"
hugo=${docforge_repo_path}/test/e2e/hugo
PR_hugo=${docforge_repo_path}/test/e2e/hugoPR
docforge_bin="${docforge_repo_path}/bin/docforge"

echo Docforge repo: "$docforge_repo_path"

echo "Clean old build if done"
rm -rf "$hugo"
rm -rf "$PR_hugo"

TOKEN=${GITHUB_OAUTH_TOKEN:-$(getGitHubToken github_com)}
test "$TOKEN" #fail fast
echo Token accepted
export GITHUB_OAUTH_TOKEN=$TOKEN

buildWebsite() {
  LOCAL_BUILD=1 "${docforge_repo_path}/.ci/build" >/dev/null 2>&1
  sudo mv "$docforge_bin" /usr/local/bin/docforge
  DOCFORGE_CONFIG="${docforge_repo_path}/test/e2e/docforge_config.yaml" docforge
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




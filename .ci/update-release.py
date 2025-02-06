#!/usr/bin/env python3

# SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

import os

import ccc.github

repo_owner_and_name = os.environ['SOURCE_GITHUB_REPO_OWNER_AND_NAME']
repo_dir = os.environ['MAIN_REPO_DIR']
output_dir = os.environ['BINARY']

repo_owner, repo_name = repo_owner_and_name.split('/')

version_file_path = os.path.join(repo_dir, 'VERSION')

with open(version_file_path) as f:
    version_file_contents = f.read()

github_api = ccc.github.github_api(repo_url=f'github.com/{repo_owner_and_name}')
repository = github_api.repository(repo_owner, repo_name)

gh_release = repository.release_from_tag(version_file_contents)

for dir, dirs, files in os.walk(os.path.join(output_dir, 'bin', 'rel')):
    for binName in files:
        with open(os.path.join(dir, binName), 'rb') as f:
            gh_release.upload_asset(
                content_type='application/octet-stream',
                name=f'{binName}',
                asset=f,
            )

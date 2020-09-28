#!/usr/bin/env python3

import pathlib
import util
import os

from github.util import GitHubRepositoryHelper

VERSION_FILE_NAME='VERSION'

repo_owner_and_name = util.check_env('SOURCE_GITHUB_REPO_OWNER_AND_NAME')
repo_dir = util.check_env('MAIN_REPO_DIR')
output_dir = util.check_env('BINARY_PATH')

repo_owner, repo_name = repo_owner_and_name.split('/')

repo_path = pathlib.Path(repo_dir).resolve()
output_path = pathlib.Path(output_dir).resolve()
version_file_path = repo_path / VERSION_FILE_NAME

version_file_contents = version_file_path.read_text()

cfg_factory = util.ctx().cfg_factory()
github_cfg = cfg_factory.github('github_com')

github_repo_helper = GitHubRepositoryHelper(
    owner=repo_owner,
    name=repo_name,
    github_cfg=github_cfg,
)

gh_release = github_repo_helper.repository.release_from_tag(version_file_contents)

for dir, dirs, files in os.walk(os.path.join(output_path, "bin", "rel")):
    for binName in files:
        dir_path = pathlib.Path(dir).resolve()
        binFilePath = dir_path / binName
        gh_release.upload_asset(
            content_type='application/octet-stream',
            name=f'{binName}',
            asset=binFilePath.open(mode='rb'),
        )
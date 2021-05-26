// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gardener/docforge/pkg/git"
	ghrh "github.com/gardener/docforge/pkg/resourcehandlers/github"

	"github.com/google/go-github/v32/github"
)

func ReadGitInfo(ctx context.Context, uri string, rl *ghrh.ResourceLocator) ([]byte, error) {
	var (
		log  []*GitLogEntry
		blob []byte
		err  error
	)

	// TODO: move to cmd validation
	if !checkGitExists() {
		return nil, fmt.Errorf("reading Git info for %s failed: git not found in PATH", uri)
	}

	if log, err = GitLog(uri); err != nil {
		return nil, err
	}

	if len(log) == 0 {
		return nil, nil
	}

	for _, logEntry := range log {
		logEntry.Name = strings.Split(logEntry.Name, "<")[0]
		logEntry.Name = strings.TrimSpace(logEntry.Name)
	}
	authorName := log[len(log)-1].Name
	authorEmail := log[len(log)-1].Email
	publishD := log[len(log)-1].Date
	lastModD := log[0].Date
	gitInfo := &git.GitInfo{
		PublishDate:      &publishD,
		LastModifiedDate: &lastModD,
		Author: &github.User{
			Name:  &authorName,
			Email: &authorEmail,
		},
		Contributors: []*github.User{},
	}

	if rl != nil {
		s := fmt.Sprintf("%s://%s", rl.Scheme, rl.Host)
		webURLElements := []string{s, rl.Owner, rl.Repo}
		webURL := strings.Join(webURLElements, "/")
		gitInfo.WebURL = &webURL
		gitInfo.SHAAlias = &rl.SHAAlias
		gitInfo.Path = &rl.Path
	}

	for _, logEntry := range log {
		if logEntry.Email != *gitInfo.Author.Email {
			name := logEntry.Name
			email := logEntry.Email
			gitInfo.Contributors = append(gitInfo.Contributors, &github.User{
				Name:  &name,
				Email: &email,
			})
		}
	}

	if blob, err = json.MarshalIndent(gitInfo, "", "  "); err != nil {
		return nil, err
	}

	return blob, nil
}

func GitLog(path string) ([]*GitLogEntry, error) {
	var (
		log            []byte
		err            error
		errStr         string
		stdout, stderr bytes.Buffer

		dirPath = path
	)

	fileInfo, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	path, err = filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if !fileInfo.IsDir() {
		dirPath = filepath.Dir(path)
	}
	// TODO: temporarly use %f for commit subject instead of %s to prevent JSON parse failure when the commit message has quotes inside
	git := exec.Command("git", "-C", dirPath, "log", "--date=short", `--pretty=format:'{%n  "sha": "%H",%n  "author": "%aN <%aE>",%n  "date": "%ad",%n  "message": "%f",%n  "email": "%aE",%n  "name": "%aN"%n },'`, "--follow", path)
	git.Stdout = &stdout
	git.Stderr = &stderr
	if err = git.Run(); err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			return nil, err
		}
	}
	log, errStr = stdout.Bytes(), stderr.String()
	if len(errStr) > 0 {
		return nil, fmt.Errorf("failed to execute git log for %s:\n%s", path, errStr)
	}

	logS := string(log)
	logS = strings.ReplaceAll(logS, "'{", "{")
	logS = strings.ReplaceAll(logS, "},'", "},")
	if strings.HasSuffix(logS, ",") {
		logS = logS[:len(logS)-1]
	}
	logS = fmt.Sprintf("[%s]", logS)

	gitLog := []*GitLogEntry{}
	if err := json.Unmarshal([]byte(logS), &gitLog); err != nil {
		return nil, fmt.Errorf("failed parsing git log to JSON: %v", err)
	}
	return gitLog, nil
}

type GitLogEntry struct {
	Sha     string
	Author  string
	Date    string
	Message string
	Email   string
	Name    string
}

func checkGitExists() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

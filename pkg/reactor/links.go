// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package reactor

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"

	"github.com/gardener/docforge/pkg/api"
	"github.com/gardener/docforge/pkg/util/urls"
	"k8s.io/klog/v2"
)

// MatchForLinkRewrite tries recursively from this node
// up to the hierarchy root link rewrite rules attached
// to nodes and finally defined globally that match this
// URL to apply them and rewrite the link or return it
// untouched.
func MatchForLinkRewrite(absLink string, node *api.Node, globalRenameRules map[string]*api.LinkRewriteRule) (version *string, destination *string, text *string, title *string, isMatched bool) {
	// first try the global rules. node rules, if any will overwrite
	version, destination, text, title, isMatched = matchForLinkRewrite(absLink, version, destination, text, title, globalRenameRules)
	nodes := node.Parents()
	nodes = append(nodes, node)
	for _, node := range nodes {
		if l := node.Links; l != nil {
			if version, destination, text, title, isMatched = matchForLinkRewrite(absLink, version, destination, text, title, node.Links.Rewrites); isMatched && destination != nil && len(*destination) == 0 {
				// we got a destroy link rule. quit right here
				return
			}
			if destination != nil && version != nil && text != nil && title == nil {
				return
			}
		}
	}
	return
}

func matchForLinkRewrite(absLink string, _version, _destination, _text, _title *string, rules map[string]*api.LinkRewriteRule) (version *string, destination *string, text *string, title *string, isMatched bool) {
	var (
		regex *regexp.Regexp
		err   error
	)
	for expr, rule := range rules {
		if regex, err = regexp.Compile(expr); err != nil {
			klog.Warningf("invalid link rewrite expression: %s, %s", expr, err.Error())
			continue
		}
		if regex.Match([]byte(absLink)) {
			isMatched = true
			if rule == nil {
				empty := ""
				return nil, &empty, &empty, nil, true
			}
			version = _version
			if rule.Version != nil {
				version = rule.Version
			}
			destination = _destination
			if rule.Destination != nil {
				destination = rule.Destination
			}
			text = _text
			if rule.Text != nil {
				text = rule.Text
			}
			title = _title
			if rule.Title != nil {
				title = rule.Title
			}
		}
	}
	return
}

// MatchForDownload returns true if the provided URL is in the defined download scope
// for the node, and the resource name to use when serializing it.
func MatchForDownload(url *urls.URL, node *api.Node, globalDownloadRules *api.Downloads) (downloadResourceName string, isMatched bool) {
	downloads := []*api.Downloads{}
	if globalDownloadRules != nil {
		downloads = append(downloads, globalDownloadRules)
	}
	nodes := node.Parents()
	nodes = append(nodes, node)
	for _, p := range nodes {
		if l := p.Links; l != nil {
			if l.Downloads != nil {
				downloads = append(downloads, l.Downloads)
			}
		}
	}
	for i := len(downloads) - 1; i >= 0; i-- {
		d := downloads[i]
		if downloadResourceName, isMatched = matchForDownload(url, d); isMatched {
			return
		}
	}
	return "", false
}

func matchForDownload(url *urls.URL, downloadRules *api.Downloads) (string, bool) {
	var (
		regex                *regexp.Regexp
		downloadResourceName string
		err                  error
	)
	if downloadRules == nil {
		return "", false
	}
	link := url.String()
	for linkMatchExpr, linkRenameRules := range downloadRules.Scope {
		if regex, err = regexp.Compile(linkMatchExpr); err != nil {
			klog.Warningf("invalid link rewrite expression: %s, %s", linkMatchExpr, err.Error())
			continue
		}
		if regex.Match([]byte(link)) {
			// check for match scope-specific rules for renaming downloads first
			if renameRule := matchDownloadRenameRule(link, linkRenameRules); len(renameRule) > 0 {
				downloadResourceName = expandVariables(url, renameRule)
				return downloadResourceName, true
			}
			// check for match scope-agnostic, global rules for renaming downloads
			if renameRule := matchDownloadRenameRule(link, downloadRules.Renames); len(renameRule) > 0 {
				downloadResourceName = expandVariables(url, renameRule)
				return downloadResourceName, true
			}
			// default download resource name
			downloadResourceName := expandVariables(url, "$name_$hash$ext")
			return downloadResourceName, true
		}
	}
	// check for match scope-agnostic, global rules for renaming downloads
	if renameRule := matchDownloadRenameRule(link, downloadRules.Renames); len(renameRule) > 0 {
		downloadResourceName = expandVariables(url, renameRule)
		return downloadResourceName, true
	}
	return "", false
}

func matchDownloadRenameRule(link string, rules map[string]string) string {
	var (
		renameRegex *regexp.Regexp
		err         error
	)
	for linkRenameMatchExpr, renameRule := range rules {
		if renameRegex, err = regexp.Compile(linkRenameMatchExpr); err != nil {
			klog.Warningf("invalid link rewrite expression: %s, %s", linkRenameMatchExpr, err.Error())
			continue
		}
		if renameRegex.Match([]byte(link)) {
			return renameRule
		}
	}
	return ""
}

func expandVariables(url *urls.URL, renameExpr string) string {
	hash := md5.Sum([]byte(url.String()))
	s := renameExpr
	s = strings.ReplaceAll(s, "$name", url.ResourceName)
	s = strings.ReplaceAll(s, "$hash", hex.EncodeToString(hash[:])[:6])
	s = strings.ReplaceAll(s, "$ext", fmt.Sprintf(".%s", url.Extension))
	return s
}

func resolveNodeLinks(node *api.Node, globaLinks *api.Links) (links []*api.Links) {
	nodes := node.Parents()
	nodes = append(nodes, node)
	links = []*api.Links{}
	for i := len(nodes) - 1; i >= 0; i-- {
		node = nodes[i]
		if l := node.Links; l != nil {
			links = append(links, l)
		}
	}
	if globaLinks != nil {
		links = append(links, globaLinks)
	}
	return links
}

// TODO: code below is alternative experiment to handle
// matching link rewrite rules for a link based on merging
// all rules on each node, instead of calculating them dynamically
// up the parents chain.
// Problem unsolved is the priority of rules - they are merged
// and not ordered.
func resolveLinks(links *api.Links, nodes []*api.Node) {
	for _, n := range nodes {
		n.Links = mergeLinks(links, n.Links)
		resolveLinks(n.Links, n.Nodes)
		continue
	}
}

func mergeLinks(a, b *api.Links) *api.Links {
	if b == nil {
		return a
	}
	if a == nil {
		return b
	}
	a.Rewrites = mergeRewrites(a.Rewrites, b.Rewrites)
	a.Downloads = mergeDownloads(a.Downloads, b.Downloads)
	return a
}

func mergeRewrites(a, b map[string]*api.LinkRewriteRule) map[string]*api.LinkRewriteRule {
	if len(b) == 0 {
		return a
	}
	if len(a) == 0 {
		return b
	}
	for k, v := range b {
		if rule, ok := a[k]; ok {
			a[k] = mergeLinkRewriteRule(rule, v)
			continue
		}
		a[k] = v
	}
	return a
}

func mergeLinkRewriteRule(a, b *api.LinkRewriteRule) *api.LinkRewriteRule {
	if b.Version != nil {
		a.Version = b.Version
	}
	if b.Destination != nil {
		a.Destination = b.Destination
	}
	if b.Text != nil {
		a.Text = b.Text
	}
	if b.Title != nil {
		a.Title = b.Title
	}
	return a
}

func mergeDownloads(a, b *api.Downloads) *api.Downloads {
	if b == nil {
		return a
	}
	if a == nil {
		return b
	}
	a.Renames = mergeResourceRenameRule(a.Renames, b.Renames)
	a.Scope = mergeDownloadScope(a.Scope, b.Scope)
	return a
}

func mergeResourceRenameRule(a, b api.ResourceRenameRules) api.ResourceRenameRules {
	if len(b) == 0 {
		return a
	}
	if len(a) == 0 {
		return b
	}
	for k, v := range b {
		a[k] = v
	}
	return a
}

func mergeDownloadScope(a, b map[string]api.ResourceRenameRules) map[string]api.ResourceRenameRules {
	if len(b) == 0 {
		return a
	}
	if len(a) == 0 {
		return b
	}
	for k, v := range b {
		if rule, ok := a[k]; ok {
			a[k] = mergeResourceRenameRule(rule, v)
			continue
		}
		a[k] = v
	}
	return a
}

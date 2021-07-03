/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package krepostats

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	githubql "github.com/shurcooL/githubv4"
	"k8s.io/klog"
	"k8s.io/test-infra/prow/github"
)

var owners = []string{
	"geojaz",
	"hakman",
	"johngmyers",
	"justinsb",
	"kashifsaadat",
	"mikesplain",
	"olemarkus",
	"rdrgmnzs",
	"rifelpet",
	"zetaab",
}

type KRepoStats struct {
	GHC github.Client
}

type pullRequest struct {
	Number githubql.Int
	Author struct {
		Login githubql.String
	}
}

var lgtmRegex = regexp.MustCompile("(?m)^/lgtm\\s*$")
var approveRegex = regexp.MustCompile("(?m)^/approve\\s*$")

type searchQuery struct {
	RateLimit struct {
		Cost      githubql.Int
		Remaining githubql.Int
	}
	Search struct {
		PageInfo struct {
			HasNextPage githubql.Boolean
			EndCursor   githubql.String
		}
		Nodes []struct {
			PullRequest pullRequest `graphql:"... on PullRequest"`
		}
	} `graphql:"search(type: ISSUE, first: 100, after: $searchCursor, query: $query)"`
}

func (k *KRepoStats) Run() {
	ownersmap := map[string]bool{}
	for _, owner := range owners {
		ownersmap[owner] = true
	}

	numAuthors := map[string]int{}
	numApprovals := map[string]int{}
	numReviews := map[string]int{}

	var query bytes.Buffer
	fmt.Fprint(&query, "is:pr repo:kubernetes/kops updated:2020-07-01..2021-07-01")

	var ret []pullRequest
	vars := map[string]interface{}{
		"query":        githubql.String(query.String()),
		"searchCursor": (*githubql.String)(nil),
	}
	var totalCost int
	var remaining int
	for {
		sq := searchQuery{}
		if err := k.GHC.Query(context.TODO(), &sq, vars); err != nil {
			klog.Fatalf("query failed: %v", err)
		}
		totalCost += int(sq.RateLimit.Cost)
		remaining = int(sq.RateLimit.Remaining)
		for _, n := range sq.Search.Nodes {
			ret = append(ret, n.PullRequest)
		}
		if !sq.Search.PageInfo.HasNextPage {
			break
		}
		vars["searchCursor"] = githubql.NewString(sq.Search.PageInfo.EndCursor)
	}
	klog.Infof("Search cost %d point(s). %d remaining.", totalCost, remaining)

	for _, pr := range ret {
		numAuthors[string(pr.Author.Login)]++
		reviewers := map[string]bool{}
		approvers := map[string]bool{}

		if ownersmap[string(pr.Author.Login)] {
			// approvers[string(pr.Author.Login)] = true
		}

		reviews, err := k.GHC.ListReviews("kubernetes", "kops", int(pr.Number))
		if err != nil {
			klog.Fatalf("list reviews on %d failed: %v", pr.Number, err)
		}
		for _, review := range reviews {
			if review.State == github.ReviewStateApproved {
				reviewers[review.User.Login] = true
				approvers[review.User.Login] = true
			}
			if review.State == github.ReviewStateChangesRequested {
				reviewers[review.User.Login] = true
			}
		}

		comments, err := k.GHC.ListPullRequestComments("kubernetes", "kops", int(pr.Number))
		if err != nil {
			klog.Fatalf("list comments on %d failed: %v", pr.Number, err)
		}
		for _, comment := range comments {
			if lgtmRegex.MatchString(comment.Body) {
				reviewers[comment.User.Login] = true
			}
			if approveRegex.MatchString(comment.Body) {
				approvers[comment.User.Login] = true
			}
		}

		klog.Infof("PR: %5d %s %s %s", pr.Number, pr.Author.Login, joinkeys(approvers), joinkeys(reviewers))
		for k := range approvers {
			numApprovals[k]++
		}
		for k := range reviewers {
			numReviews[k]++
		}
	}

	printTable("authors", numAuthors)
	printTable("approvers", numApprovals)
	printTable("reviewers", numReviews)
}

func joinkeys(m map[string]bool) string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	return strings.Join(keys, ",")
}

func printTable(hdr string, t map[string]int) {
	klog.Infof("%s:", hdr)
	var val []string
	for k := range t {
		val = append(val, k)
	}
	sort.Slice(val, func(i, j int) bool {
		if t[val[i]] < t[val[j]] {
			return false
		}
		if t[val[i]] > t[val[j]] {
			return true
		}
		return val[i] < val[j]
	})
	for _, k := range val {
		klog.Infof("%6d %s", t[k], k)
	}
}

//go:build linux

package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
)

const (
	dockerHubSearchURL = "https://hub.docker.com/v2/search/repositories/"
	dockerHubTagsURL   = "https://hub.docker.com/v2/repositories/%s/tags?page_size=100"
)

type searchResult struct {
	Count   int              `json:"count"`
	Results []searchRepoItem `json:"results"`
}

type searchRepoItem struct {
	Name        string `json:"repo_name"`
	Description string `json:"short_description"`
	Stars       int    `json:"star_count"`
	Pulls       int64  `json:"pull_count"`
	Official    bool   `json:"is_official"`
}

type tagsResponse struct {
	Results []tagItem `json:"results"`
}

type tagItem struct {
	Name string `json:"name"`
}

func isPopularTag(name string) bool {
	if name == "latest" || name == "slim" || name == "alpine" || name == "bookworm" || name == "bullseye" {
		return true
	}
	if strings.HasSuffix(name, "-slim") || strings.HasSuffix(name, "-alpine") {
		return true
	}
	if strings.HasSuffix(name, "-bookworm") || strings.HasSuffix(name, "-bullseye") {
		return true
	}
	if strings.Count(name, ".") <= 1 && !strings.Contains(name, "-") {
		return true
	}
	return false
}

func fetchTags(repo string, filter string) []string {
	u := fmt.Sprintf(dockerHubTagsURL, repo)
	resp, err := http.Get(u)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	var tr tagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return nil
	}

	var popular, others []string
	for _, t := range tr.Results {
		n := t.Name
		if n == "latest" {
			continue
		}
		if filter != "" && !strings.Contains(n, filter) {
			continue
		}
		if isPopularTag(n) {
			popular = append(popular, n)
		} else {
			others = append(others, n)
		}
	}

	sort.Slice(popular, func(i, j int) bool {
		return len(popular[i]) < len(popular[j])
	})

	all := append(popular, others...)
	if len(all) > 10 {
		all = all[:10]
	}
	return all
}

func Search(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: cardinal search <term>")
		fmt.Fprintln(os.Stderr, "       cardinal search python:3.11  (filter by tag)")
		os.Exit(1)
	}

	query := strings.Join(args, " ")
	imageName := query
	tagFilter := ""

	if idx := strings.Index(query, ":"); idx > 0 {
		imageName = query[:idx]
		tagFilter = query[idx+1:]
	}

	u := fmt.Sprintf("%s?query=%s&page_size=10", dockerHubSearchURL, url.QueryEscape(imageName))

	resp, err := http.Get(u)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error searching: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		fmt.Fprintf(os.Stderr, "Error: Docker Hub returned HTTP %d\n", resp.StatusCode)
		os.Exit(1)
	}

	var sr searchResult
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing response: %v\n", err)
		os.Exit(1)
	}

	if sr.Count == 0 {
		fmt.Println("No results found")
		return
	}

	label := query
	if tagFilter != "" {
		label = fmt.Sprintf("%s (filtering tags by \"%s\")", imageName, tagFilter)
	}
	fmt.Printf("Found %d results for \"%s\". Top tags:\n\n", sr.Count, label)

	type repoWithTags struct {
		item searchRepoItem
		tags []string
	}

	results := make([]repoWithTags, len(sr.Results))
	for i, r := range sr.Results {
		results[i] = repoWithTags{item: r}
	}

	var wg sync.WaitGroup
	lim := make(chan struct{}, 5)
	for i := range results {
		wg.Add(1)
		go func(r *repoWithTags) {
			defer wg.Done()
			lim <- struct{}{}
			defer func() { <-lim }()
			repo := r.item.Name
			if r.item.Official {
				repo = "library/" + repo
			}
			r.tags = fetchTags(repo, tagFilter)
		}(&results[i])
	}
	wg.Wait()

	for _, r := range results {
		if tagFilter != "" && len(r.tags) == 0 {
			continue
		}
		official := ""
		if r.item.Official {
			official = " [official]"
		}
		desc := strings.TrimSpace(r.item.Description)
		if len(desc) > 80 {
			desc = desc[:77] + "..."
		}
		if desc == "" {
			desc = "(no description)"
		}
		fmt.Printf("  %s%s\n", r.item.Name, official)
		fmt.Printf("    %s\n", desc)
		fmt.Printf("    Stars: %d  Pulls: %d\n", r.item.Stars, r.item.Pulls)
		if len(r.tags) > 0 {
			fmt.Printf("    Tags: %s\n", strings.Join(r.tags, ", "))
			fmt.Printf("    Use:  cardinal pull %s\n", r.item.Name+":"+r.tags[0])
		}
		fmt.Println()
	}
}

//go:build linux

package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type blueprintRepo struct {
	Name    string `json:"name"`
	URL     string `json:"url"`
	Branch  string `json:"branch"`
	Enabled bool   `json:"enabled"`
}

type blueprintRepoConfig struct {
	Repos []blueprintRepo `json:"repos"`
}

func defaultBlueprintRepo() blueprintRepo {
	return blueprintRepo{
		Name:    "official",
		URL:     "https://raw.githubusercontent.com/animesao/cardinal-blueprints",
		Branch:  "main",
		Enabled: true,
	}
}

func blueprintReposPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "/root"
	}
	return filepath.Join(home, ".cardinal", "blueprint-repos.json")
}

func loadBlueprintRepos() *blueprintRepoConfig {
	path := blueprintReposPath()
	data, err := os.ReadFile(path)
	if err != nil {
		cfg := &blueprintRepoConfig{
			Repos: []blueprintRepo{defaultBlueprintRepo()},
		}
		if err := saveBlueprintRepos(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving default blueprint repository config: %v\n", err)
		}
		return cfg
	}

	var cfg blueprintRepoConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		cfg = blueprintRepoConfig{Repos: []blueprintRepo{defaultBlueprintRepo()}}
	}
	if len(cfg.Repos) == 0 {
		cfg.Repos = []blueprintRepo{defaultBlueprintRepo()}
	}
	return &cfg
}

func saveBlueprintRepos(cfg *blueprintRepoConfig) error {
	path := blueprintReposPath()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func normalizeRepoURL(input string) string {
	input = strings.TrimSpace(input)
	if input == "" {
		return ""
	}
	if strings.HasPrefix(input, "https://raw.githubusercontent.com/") {
		return strings.TrimSuffix(input, "/")
	}
	if strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://") {
		input = strings.TrimSuffix(input, "/")
		if strings.Contains(input, "github.com/") {
			return strings.Replace(input, "github.com", "raw.githubusercontent.com", 1)
		}
		return input
	}
	if strings.Contains(input, "/") && !strings.Contains(input, "://") {
		return "https://raw.githubusercontent.com/" + strings.TrimSuffix(input, "/")
	}
	return input
}

func repoNameFromURL(rawURL string) string {
	rawURL = strings.TrimSuffix(rawURL, "/")
	parts := strings.Split(rawURL, "/")
	if len(parts) >= 2 {
		return parts[len(parts)-1]
	}
	return rawURL
}

func blueprintRepoAdd(args []string) {
	if len(args) < 1 || args[0] == "" {
		fmt.Println("Usage: cardinal blueprint repo add <url> [--name <name>] [--branch <branch>]")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  cardinal blueprint repo add user/my-blueprints")
		fmt.Println("  cardinal blueprint repo add https://github.com/user/my-blueprints --branch dev")
		fmt.Println("  cardinal blueprint repo add https://raw.githubusercontent.com/user/my-blueprints")
		os.Exit(1)
	}

	input := args[0]
	rawURL := normalizeRepoURL(input)
	if rawURL == "" {
		fmt.Fprintf(os.Stderr, "Invalid repository URL: %s\n", input)
		os.Exit(1)
	}

	name := ""
	branch := "main"
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--name":
			if i+1 < len(args) {
				i++
				name = args[i]
			}
		case "--branch":
			if i+1 < len(args) {
				i++
				branch = args[i]
			}
		}
	}
	if name == "" {
		name = repoNameFromURL(rawURL)
	}

	cfg := loadBlueprintRepos()
	for _, r := range cfg.Repos {
		if r.URL == rawURL && r.Branch == branch {
			fmt.Printf("Repository already exists: %s (%s, branch: %s)\n", r.Name, r.URL, r.Branch)
			return
		}
	}

	cfg.Repos = append(cfg.Repos, blueprintRepo{
		Name:    name,
		URL:     rawURL,
		Branch:  branch,
		Enabled: true,
	})
	if err := saveBlueprintRepos(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Added repository: %s\n", name)
	fmt.Printf("  URL:    %s\n", rawURL)
	fmt.Printf("  Branch: %s\n", branch)
}

func blueprintRepoList() {
	cfg := loadBlueprintRepos()
	if len(cfg.Repos) == 0 {
		fmt.Println("No blueprint repositories configured.")
		return
	}
	fmt.Println("Blueprint repositories:")
	fmt.Println()
	for i, r := range cfg.Repos {
		status := "enabled"
		if !r.Enabled {
			status = "disabled"
		}
		fmt.Printf("  [%d] %s (%s)\n", i, r.Name, status)
		fmt.Printf("      URL:    %s\n", r.URL)
		fmt.Printf("      Branch: %s\n", r.Branch)
		fmt.Println()
	}
}

func blueprintRepoRemove(args []string) {
	if len(args) < 1 || args[0] == "" {
		fmt.Println("Usage: cardinal blueprint repo remove <name|url|index>")
		os.Exit(1)
	}
	target := args[0]

	cfg := loadBlueprintRepos()
	found := -1
	for i, r := range cfg.Repos {
		if r.Name == target || r.URL == target || r.URL == normalizeRepoURL(target) || fmt.Sprintf("%d", i) == target {
			found = i
			break
		}
	}
	if found < 0 {
		fmt.Fprintf(os.Stderr, "Repository %q not found\n", target)
		os.Exit(1)
	}

	removed := cfg.Repos[found]
	cfg.Repos = append(cfg.Repos[:found], cfg.Repos[found+1:]...)
	if err := saveBlueprintRepos(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Removed repository: %s (%s)\n", removed.Name, removed.URL)
}

func blueprintRepoUsage() {
	fmt.Println(`Repository commands:
  cardinal blueprint repo list                 List configured blueprint repositories
  cardinal blueprint repo add <url> [options]  Add a blueprint repository
  cardinal blueprint repo remove <name>        Remove a blueprint repository

Add options:
  --name <name>     Display name (default: derived from URL)
  --branch <name>   Git branch (default: main)

URL formats accepted:
  user/repo
  https://github.com/user/repo
  https://raw.githubusercontent.com/user/repo`)
}

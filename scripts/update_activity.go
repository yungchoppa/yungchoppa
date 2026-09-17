package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Event struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	CreatedAt time.Time `json:"created_at"`
	Repo      struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	} `json:"repo"`
	Payload struct {
		Action  string `json:"action"`
		Ref     string `json:"ref"`
		RefType string `json:"ref_type"`
		Head    string `json:"head"`
		Commits []struct {
			SHA     string `json:"sha"`
			Message string `json:"message"`
			URL     string `json:"url"`
		} `json:"commits"`
		Issue struct {
			Number  int    `json:"number"`
			Title   string `json:"title"`
			HTMLURL string `json:"html_url"`
		} `json:"issue"`
		PullRequest struct {
			Number  int    `json:"number"`
			Title   string `json:"title"`
			HTMLURL string `json:"html_url"`
			Merged  bool   `json:"merged"`
		} `json:"pull_request"`
		Comment struct {
			HTMLURL string `json:"html_url"`
		} `json:"comment"`
	} `json:"payload"`
}

type SearchCommitResult struct {
	TotalCount int `json:"total_count"`
	Items      []struct {
		SHA     string `json:"sha"`
		HTMLURL string `json:"html_url"`
		Commit  struct {
			Author struct {
				Name string    `json:"name"`
				Date time.Time `json:"date"`
			} `json:"author"`
			Committer struct {
				Name string    `json:"name"`
				Date time.Time `json:"date"`
			} `json:"committer"`
			Message string `json:"message"`
		} `json:"commit"`
		Repository struct {
			Name     string `json:"name"`
			FullName string `json:"full_name"`
			HTMLURL  string `json:"html_url"`
		} `json:"repository"`
	} `json:"items"`
}

type ProjectContribution struct {
	RepoFullName string
	RepoURL      string
	Language     string
	LangBadge    string
	CommitsCount int
	GerritCount  int
	PRCount      int
	IssueCount   int
	ReviewCount  int
	Items        []string
}

const (
	startMarker   = "<!--START_SECTION:contributions-->"
	endMarker     = "<!--END_SECTION:contributions-->"
	maxPerProject = 10
)

func newHTTPClient() *http.Client {
	tr := &http.Transport{
		TLSClientConfig:   &tls.Config{},
		DisableKeepAlives: true,
	}
	return &http.Client{
		Transport: tr,
		Timeout:   20 * time.Second,
	}
}

func main() {
	username := os.Getenv("GITHUB_ACTOR")
	if username == "" {
		username = "yungchoppa"
	}
	if len(os.Args) > 1 && os.Args[1] != "" {
		username = os.Args[1]
	}

	readmePath := "README.md"
	if len(os.Args) > 2 && os.Args[2] != "" {
		readmePath = os.Args[2]
	}

	fmt.Printf("Fetching public events for user '%s'...\n", username)
	events, err := fetchUserEvents(username)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Error fetching events: %v\n", err)
	}

	fmt.Printf("Fetching authored commits for user '%s'...\n", username)
	commits, err := fetchUserCommits(username)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Error fetching authored commits: %v\n", err)
	}

	projects := aggregateProjects(username, events, commits)

	content := buildContributionsMarkdown(projects)
	fmt.Println("Generated aligned project headers with collapsible activity.")

	err = updateReadme(readmePath, content)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error updating %s: %v\n", readmePath, err)
		os.Exit(1)
	}

	fmt.Printf("Successfully updated %s!\n", readmePath)
}

func fetchUserEvents(username string) ([]Event, error) {
	url := fmt.Sprintf("https://api.github.com/users/%s/events/public?per_page=50", username)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "yungchoppa-activity-updater")
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "token "+token)
	}

	client := newHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API responded with status: %s", resp.Status)
	}

	var events []Event
	if err := json.NewDecoder(resp.Body).Decode(&events); err != nil {
		return nil, err
	}

	return events, nil
}

func fetchUserCommits(username string) (*SearchCommitResult, error) {
	url := fmt.Sprintf("https://api.github.com/search/commits?q=author:%s&sort=author-date&order=desc&per_page=50", username)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "yungchoppa-activity-updater")
	req.Header.Set("Accept", "application/vnd.github.cloak-preview+json")

	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "token "+token)
	}

	client := newHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API responded with status: %s", resp.Status)
	}

	var result SearchCommitResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

func aggregateProjects(username string, events []Event, commitSearch *SearchCommitResult) []ProjectContribution {
	// 1. golang/tools project
	golangProject := ProjectContribution{
		RepoFullName: "golang/tools",
		RepoURL:      "https://github.com/golang/tools",
		Language:     "Go",
		LangBadge:    "https://img.shields.io/badge/Go-00ADD8?style=flat-square&logo=go&logoColor=white",
		CommitsCount: 1,
		GerritCount:  1,
		IssueCount:   1,
		ReviewCount:  2,
		Items: []string{
			"- 🚀 **Commit** [`3e95368`](https://github.com/golang/tools/commit/3e95368d18d5e4e9076b59577200829fb9eb4e17) • [Gerrit CL 831004](https://go-review.googlesource.com/c/tools/+/831004) • [Issue #80845](https://github.com/golang/go/issues/80845) — Support inlay hints for partial generic type arguments in `gopls` *(11 Sep 2026)*",
		},
	}

	// 2. Horuse/Splitwave project
	splitwaveProject := ProjectContribution{
		RepoFullName: "Horuse/Splitwave",
		RepoURL:      "https://github.com/Horuse/Splitwave",
		Language:     "Rust",
		LangBadge:    "https://img.shields.io/badge/Rust-DEA584?style=flat-square&logo=rust&logoColor=black",
		PRCount:      1,
		IssueCount:   1,
		ReviewCount:  1,
		Items: []string{
			"- ✨ **PR** [#36](https://github.com/Horuse/Splitwave/pull/36) • [Issue #35 (Resolved)](https://github.com/Horuse/Splitwave/issues/35) — Fix host DAW deadlocks & GUI crashes via Tao/WebView sync *(03 Sep 2026)*",
			"- 🗣️ **Discussion** [Issue #35](https://github.com/Horuse/Splitwave/issues/35#issuecomment-5553411485) — Technical investigation & debugging of `wry` event loop in VST3 architectures *(05 Sep 2026)*",
		},
	}

	// Append any additional dynamic commits for golang/tools or splitwave from search
	seen := make(map[string]bool)
	seen["3e95368"] = true

	if commitSearch != nil {
		for _, c := range commitSearch.Items {
			shortSHA := c.SHA
			if len(shortSHA) > 7 {
				shortSHA = shortSHA[:7]
			}
			if seen[c.SHA] || seen[shortSHA] {
				continue
			}

			repoName := c.Repository.FullName
			if strings.EqualFold(repoName, "golang/tools") {
				msg := strings.Split(c.Commit.Message, "\n")[0]
				dateStr := c.Commit.Author.Date.Format("02 Jan 2006")
				item := fmt.Sprintf("- 🚀 **Commit** [`%s`](%s) — %s *(%s)*", shortSHA, c.HTMLURL, escapeMarkdown(msg), dateStr)
				golangProject.Items = append(golangProject.Items, item)
				golangProject.CommitsCount++
				seen[shortSHA] = true
			} else if strings.EqualFold(repoName, "Horuse/Splitwave") {
				msg := strings.Split(c.Commit.Message, "\n")[0]
				dateStr := c.Commit.Author.Date.Format("02 Jan 2006")
				item := fmt.Sprintf("- 🚀 **Commit** [`%s`](%s) — %s *(%s)*", shortSHA, c.HTMLURL, escapeMarkdown(msg), dateStr)
				splitwaveProject.Items = append(splitwaveProject.Items, item)
				splitwaveProject.PRCount++
				seen[shortSHA] = true
			}
		}
	}

	return []ProjectContribution{golangProject, splitwaveProject}
}

func escapeMarkdown(text string) string {
	text = strings.ReplaceAll(text, "`", "'")
	return strings.TrimSpace(text)
}

func buildContributionsMarkdown(projects []ProjectContribution) string {
	var sb strings.Builder

	// Calculate max name length to align horizontally
	maxLen := 0
	for _, p := range projects {
		if len(p.RepoFullName) > maxLen {
			maxLen = len(p.RepoFullName)
		}
	}

	for _, p := range projects {
		var badges []string
		if p.CommitsCount > 0 {
			badges = append(badges, fmt.Sprintf(`<img src="https://img.shields.io/badge/Commits-%d-7aa2f7?style=flat-square" alt="Commits" valign="middle" />`, p.CommitsCount))
		}
		if p.GerritCount > 0 {
			badges = append(badges, fmt.Sprintf(`<img src="https://img.shields.io/badge/Gerrit_CL-%d-4285f4?style=flat-square" alt="Gerrit" valign="middle" />`, p.GerritCount))
		}
		if p.PRCount > 0 {
			badges = append(badges, fmt.Sprintf(`<img src="https://img.shields.io/badge/PRs-%d_Merged-8957e5?style=flat-square" alt="PRs" valign="middle" />`, p.PRCount))
		}
		if p.IssueCount > 0 {
			badges = append(badges, fmt.Sprintf(`<img src="https://img.shields.io/badge/Issues-%d_Resolved-2ea44f?style=flat-square" alt="Issues" valign="middle" />`, p.IssueCount))
		}
		if p.ReviewCount > 0 {
			badges = append(badges, fmt.Sprintf(`<img src="https://img.shields.io/badge/Reviews-%d-388bfd?style=flat-square" alt="Reviews" valign="middle" />`, p.ReviewCount))
		}
		badgesStr := strings.Join(badges, " ")

		// Add subtle em-space padding if shorter than max length
		pad := ""
		diff := maxLen - len(p.RepoFullName)
		if diff > 0 {
			// In standard font, approx 1 em-space + thin space balances 4 chars
			pad = "&emsp;&thinsp;"
		}

		limit := len(p.Items)
		if limit > maxPerProject {
			limit = maxPerProject
		}
		itemsStr := strings.Join(p.Items[:limit], "\n")

		sb.WriteString(fmt.Sprintf(`<details open>
<summary>
  <b><a href="%s">%s</a></b>%s
  &nbsp; <img src="%s" alt="%s" valign="middle" />
  &nbsp; %s
</summary>
<br>

%s

</details>

`, p.RepoURL, p.RepoFullName, pad, p.LangBadge, p.Language, badgesStr, itemsStr))
	}

	return strings.TrimSpace(sb.String())
}

func updateReadme(readmePath string, newContent string) error {
	absPath, err := filepath.Abs(readmePath)
	if err != nil {
		return err
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return err
	}

	content := string(data)
	startIdx := strings.Index(content, startMarker)
	endIdx := strings.Index(content, endMarker)

	if startIdx == -1 || endIdx == -1 {
		return fmt.Errorf("markers %s and/or %s not found in %s", startMarker, endMarker, readmePath)
	}

	if startIdx > endIdx {
		return fmt.Errorf("invalid marker ordering in %s", readmePath)
	}

	finalContent := content[:startIdx+len(startMarker)] + "\n\n" + newContent + "\n\n" + content[endIdx:]

	return os.WriteFile(absPath, []byte(finalContent), 0644)
}

package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"time"

	cu "github.com/sonichigo/hg/common"
)

func calculateDownloadStats(releases []cu.Release) *cu.DownloadStats {
	stats := &cu.DownloadStats{
		Releases: make([]cu.ReleaseDownloadStats, 0),
	}

	// Sort releases by creation date (newest first)
	sort.Slice(releases, func(i, j int) bool {
		return releases[i].CreatedAt.After(releases[j].CreatedAt)
	})

	for _, release := range releases {
		releaseStats := cu.ReleaseDownloadStats{
			TagName:   release.TagName,
			CreatedAt: release.CreatedAt,
			Assets:    make([]cu.AssetStats, 0),
		}

		for _, asset := range release.Assets {
			assetStats := cu.AssetStats{
				Name:          asset.Name,
				DownloadCount: asset.DownloadCount,
			}
			releaseStats.TotalDownloads += asset.DownloadCount
			releaseStats.Assets = append(releaseStats.Assets, assetStats)
		}

		stats.TotalDownloads += releaseStats.TotalDownloads
		stats.Releases = append(stats.Releases, releaseStats)
	}

	return stats
}

func getAllReleases(owner, repo string, config *cu.Config) ([]cu.Release, error) {
	var allReleases []cu.Release
	page := 1
	perPage := 100

	for {
		url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases?page=%d&per_page=%d",
			owner, repo, page, perPage)

		client := &http.Client{}
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("error creating request: %v", err)
		}

		// Add headers conditionally
		req.Header.Add("Accept", "application/vnd.github.v3+json")
		if config != nil && config.GithubToken != "" {
			req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", config.GithubToken))
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("error making request: %v", err)
		}

		// Check rate limit headers
		remaining := resp.Header.Get("X-RateLimit-Remaining")
		limit := resp.Header.Get("X-RateLimit-Limit")

		if resp.StatusCode == http.StatusForbidden {
			resp.Body.Close()
			return nil, fmt.Errorf("rate limit exceeded. Please use a GitHub token. Limit: %s, Remaining: %s", limit, remaining)
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("GitHub API returned status: %d, body: %s", resp.StatusCode, string(body))
		}

		var releases []cu.Release
		if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("error decoding response: %v", err)
		}
		resp.Body.Close()

		if len(releases) == 0 {
			break
		}

		allReleases = append(allReleases, releases...)

		if len(releases) < perPage {
			break
		}

		page++
	}

	return allReleases, nil
}

// extractRepoInfo extracts owner and repo name from GitHub URL
func extractRepoInfo(repoURL string) (string, string, error) {
	patterns := []string{
		`github\.com[:/]([^/]+)/([^/\.]+)(?:\.git)?$`,
		`github\.com/([^/]+)/([^/\.]+)/?$`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(repoURL)
		if len(matches) == 3 {
			return matches[1], matches[2], nil
		}
	}

	return "", "", fmt.Errorf("invalid GitHub repository URL")
}
func getStarHistory(owner, repo string, config *cu.Config) (*cu.StarHistory, error) {
	// GitHub's API doesn't provide direct star history, so we'll use stargazers endpoint
	page := 1
	perPage := 100
	history := make([]cu.StarPoint, 0)

	for {
		url := fmt.Sprintf("https://api.github.com/repos/%s/%s/stargazers?page=%d&per_page=%d",
			owner, repo, page, perPage)

		client := &http.Client{}
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("error creating request: %v", err)
		}

		// Required header for timestamp information
		req.Header.Add("Accept", "application/vnd.github.v3.star+json")
		if config != nil && config.GithubToken != "" {
			req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", config.GithubToken))
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("error making request: %v", err)
		}

		if resp.StatusCode == http.StatusForbidden {
			resp.Body.Close()
			return nil, fmt.Errorf("rate limit exceeded. Please use a GitHub token")
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("GitHub API returned status: %d, body: %s", resp.StatusCode, string(body))
		}

		// Parse response
		var stargazers []struct {
			StarredAt time.Time `json:"starred_at"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&stargazers); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("error decoding response: %v", err)
		}
		resp.Body.Close()

		if len(stargazers) == 0 {
			break
		}

		// Accumulate star counts
		starCount := page * perPage
		for i, sg := range stargazers {
			history = append(history, cu.StarPoint{
				Date:  sg.StarredAt,
				Stars: starCount - (len(stargazers) - i - 1),
			})
		}

		if len(stargazers) < perPage {
			break
		}

		page++
	}

	// Sort history by date
	sort.Slice(history, func(i, j int) bool {
		return history[i].Date.Before(history[j].Date)
	})

	return &cu.StarHistory{
		RepoName: fmt.Sprintf("%s/%s", owner, repo),
		History:  history,
	}, nil
}

func getOrgContributors(org string, config *cu.Config) (*cu.OrganizationStats, error) {
	page := 1
	perPage := 100
	totalContributors := make(map[string]struct{})
	totalRepos := 0

	for {
		url := fmt.Sprintf("https://api.github.com/orgs/%s/repos?page=%d&per_page=%d", org, page, perPage)

		client := &http.Client{}
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("error creating request: %v", err)
		}

		req.Header.Add("Accept", "application/vnd.github.v3+json")
		if config != nil && config.GithubToken != "" {
			req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", config.GithubToken))
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("error making request: %v", err)
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("GitHub API returned status: %d, body: %s", resp.StatusCode, string(body))
		}

		var repos []map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&repos); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("error decoding response: %v", err)
		}
		resp.Body.Close()

		if len(repos) == 0 {
			break
		}

		totalRepos += len(repos)

		for _, repo := range repos {
			repoName := repo["name"].(string)
			repoContributorsURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/contributors", org, repoName)

			contribReq, err := http.NewRequest("GET", repoContributorsURL, nil)
			if err != nil {
				return nil, fmt.Errorf("error creating contributors request: %v", err)
			}

			contribReq.Header.Add("Accept", "application/vnd.github.v3+json")
			if config != nil && config.GithubToken != "" {
				contribReq.Header.Add("Authorization", fmt.Sprintf("Bearer %s", config.GithubToken))
			}

			contribResp, err := client.Do(contribReq)
			if err != nil {
				return nil, fmt.Errorf("error making contributors request: %v", err)
			}

			if contribResp.StatusCode == http.StatusNoContent {
				// Repository has no contributors, continue to the next one
				contribResp.Body.Close()
				continue
			}

			if contribResp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(contribResp.Body)
				contribResp.Body.Close()
				return nil, fmt.Errorf("GitHub API returned status for contributors: %d, body: %s", contribResp.StatusCode, string(body))
			}

			var contributors []cu.Contributor
			if err := json.NewDecoder(contribResp.Body).Decode(&contributors); err != nil {
				contribResp.Body.Close()
				return nil, fmt.Errorf("error decoding contributors response: %v", err)
			}
			contribResp.Body.Close()

			for _, contributor := range contributors {
				totalContributors[contributor.Login] = struct{}{}
			}
		}

		page++
	}

	return &cu.OrganizationStats{
		OrgName:           org,
		TotalRepos:        totalRepos,
		TotalContributors: len(totalContributors),
	}, nil
}

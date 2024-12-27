package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	cu "github.com/sonichigo/hg/common"
)

func HandleRepoStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	repoURL := r.URL.Query().Get("repo")
	if repoURL == "" {
		http.Error(w, "Repository URL is required", http.StatusBadRequest)
		return
	}

	owner, repo, err := extractRepoInfo(repoURL)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid repository URL: %v", err), http.StatusBadRequest)
		return
	}

	// Make token optional
	var config *cu.Config
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		token := strings.TrimPrefix(authHeader, "Bearer ")
		token = strings.TrimSpace(token)
		config = &cu.Config{GithubToken: token}
	}

	releases, err := getAllReleases(owner, repo, config)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	stats := calculateDownloadStats(releases)
	stats.RepoName = fmt.Sprintf("%s/%s", owner, repo)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}
func HandleStarHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get repositories from query parameter
	repos := r.URL.Query()["repo"]
	if len(repos) == 0 {
		http.Error(w, "At least one repository URL is required", http.StatusBadRequest)
		return
	}

	// Make token optional
	var config *cu.Config
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		token := strings.TrimPrefix(authHeader, "Bearer ")
		token = strings.TrimSpace(token)
		config = &cu.Config{GithubToken: token}
	}

	// Fetch star history for all repositories
	result := cu.MultiRepoStarHistory{
		Repositories: make([]cu.StarHistory, 0, len(repos)),
	}

	for _, repoURL := range repos {
		owner, repo, err := extractRepoInfo(repoURL)
		if err != nil {
			http.Error(w, fmt.Sprintf("Invalid repository URL: %v", err), http.StatusBadRequest)
			return
		}

		history, err := getStarHistory(owner, repo, config)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		result.Repositories = append(result.Repositories, *history)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
func HandleOrgContributors(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	org := r.URL.Query().Get("org")
	if org == "" {
		http.Error(w, "Organization name is required", http.StatusBadRequest)
		return
	}

	// Make token optional
	var config *cu.Config
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		token := strings.TrimPrefix(authHeader, "Bearer ")
		token = strings.TrimSpace(token)
		config = &cu.Config{GithubToken: token}
	}

	stats, err := getOrgContributors(org, config)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

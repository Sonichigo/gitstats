package routes

import (
	"net/http"

	handler "github.com/sonichigo/hg/handlers"
)

func SetupRoutes() {
	http.Handle("/images/", http.StripPrefix("/images", http.FileServer(http.Dir("./images"))))

	// Serve static files
	http.HandleFunc("/", handler.ServerIndex)
	http.HandleFunc("/orgs", handler.ServerOrgPage)
	http.HandleFunc("/starhistory", handler.ServerStartPage)

	// API endpoint
	http.HandleFunc("/repo-stats", handler.HandleRepoStats)
	http.HandleFunc("/org-contributors", handler.HandleOrgContributors)
	http.HandleFunc("/star-history", handler.HandleStarHistory)
}

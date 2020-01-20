package main

import (
	"github.com/hashicorp/go-getter"
	"github.com/hjaensch7/webhooks/github"
)

// Build download url
func getHTTPDownloadURLPush(p github.PushPayload) string {
	return "git::" + p.Repository.URL
}

func getHTTPDownloadURLInstallation(p github.InstallationPayload) string {
	return "git::" + "https://github.com/" + p.Repositories[0].FullName
}

func getHTTPDownloadURLInstallationRepositories(p github.InstallationRepositoriesPayload) string {
	return "git::" + "https://github.com/" + p.RepositoriesAdded[0].FullName
}

// Download repo contents to a specific location
func downloadRepo(downloadURL string, downloadLocation string) error {
	err := getter.Get(downloadLocation, downloadURL)
	if err != nil {
		return err
	}
	return nil
}

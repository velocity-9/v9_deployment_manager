package main

import (
	"github.com/hashicorp/go-getter"
	"gopkg.in/go-playground/webhooks.v5/github"
)

// Build download url
func getHTTPDownloadURL(p github.PushPayload) string {
	return "git::" + p.Repository.URL
}

// Download repo contents to a specific location
func downloadRepo(downloadURL string, downloadLocation string) error {
	err := getter.Get(downloadLocation, downloadURL)
	if err != nil {
		return err
	}
	return nil
}

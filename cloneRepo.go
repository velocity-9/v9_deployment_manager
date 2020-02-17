package main

import (
	"bytes"
	"io/ioutil"
	"log"
	"os/exec"

	"v9_deployment_manager/log"

	"gopkg.in/src-d/go-git.v4"
)

//Clone repo into temp dir
func cloneRepo(repoName string) (dirName string, err error) {
	// Tempdir to clone the repository
	dir, err := ioutil.TempDir("", ".git_")
	if err != nil {
		log.Error.Println(err)
	}

	_, err = git.PlainClone(dir, false, &git.CloneOptions{
		URL: "https://github.com/" + repoName + ".git",
	})

	if err != nil {
		log.Error.Println(err)
	}
	return dir, err
}

func getHash(repoFilePathAbs string) (hash string, err error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repoFilePathAbs
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	hash = stdout.Bytes().String()
	hash = hash[:len(hash)-1]
	return hash, err
}

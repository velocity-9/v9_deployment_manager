package main

import (
	"bytes"
	"io/ioutil"
	"log"
	"os/exec"

	"gopkg.in/src-d/go-git.v4"
)

//Clone repo into temp dir
func cloneRepo(repoName string) (dir_name string, err error) {
	// Tempdir to clone the repository
	dir, err := ioutil.TempDir("", ".git_")
	if err != nil {
		log.Fatal(err) //FIXME correct this
	}

	_, err = git.PlainClone(dir, false, &git.CloneOptions{
		URL: "https://github.com/" + repoName + ".git",
	})

	if err != nil {
		log.Fatal(err) //FIXME correct
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
	hash = string(stdout.Bytes())
	hash = hash[:len(hash)-1]
	return hash, err
}

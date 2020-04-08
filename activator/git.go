package activator

import (
	"bytes"
	"io/ioutil"
	"os/exec"

	"v9_deployment_manager/log"
	"v9_deployment_manager/worker"

	"gopkg.in/src-d/go-git.v4"
)

//Checkout head of specific repo
func checkout(path string, hash string) error {
	cmd := exec.Command("git", "checkout", hash)
	cmd.Dir = path
	return cmd.Run()
}

//Clone repo into temp dir
func cloneRepo(repoName string) (string, error) {
	// Tempdir to clone the repository
	dir, err := ioutil.TempDir("", ".git_")
	if err != nil {
		log.Error.Println(err)
		return "", err
	}

	// TODO: Don't hardcode Github here
	_, err = git.PlainClone(dir, false, &git.CloneOptions{
		URL: "https://github.com/" + repoName + ".git",
	})

	if err != nil {
		log.Error.Println(err)
		return "", err
	}
	return dir, err
}

func getHash(repoFilePathAbs string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repoFilePathAbs
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return "", err
	}
	hash := stdout.String()
	hash = hash[:len(hash)-1]
	return hash, err
}

type cloneResult struct {
	path string
	hash string
}

func cloneAndSetHash(compID worker.ComponentID) (cloneResult, error) {
	fullRepoName := compID.User + "/" + compID.Repo
	// Get Repo Contents
	log.Info.Println("Cloning " + compID.Repo + "...")
	clonedPath, err := cloneRepo(fullRepoName)
	if err != nil {
		log.Error.Println("Error cloning repo:", err)
		return cloneResult{}, err
	}

	err = checkout(clonedPath, compID.Hash)
	if err != nil {
		log.Error.Println("git checkout HEAD failed", err)
	}

	if compID.Hash == "HEAD" {
		compID.Hash, err = getHash(clonedPath)
		if err != nil {
			log.Error.Println("Error getting hash from repo:", err)
			return cloneResult{}, err
		}
	}

	return cloneResult{
		path: clonedPath,
		hash: compID.Hash,
	}, nil
}

package main

import (
	"os"
	"os/exec"
)

// Build Docker Image Based on Dockerfile
func buildImageFromDockerfile(tarName string, tempRepoPath string) error {
	cmd := exec.Command("docker", "build", "-t", tarName, tempRepoPath)
	return cmd.Run()
}

// Build .tar from Docker Image
func buildTarFromImage(tarName string) error {
	tarNameExt := tarName + ".tar"
	cmd := exec.Command("docker", "save", tarName, "-o", tarNameExt)
	return cmd.Run()
}

// GZip tar
func gzipTar(tarName string) error {
	cmd := exec.Command("gzip", tarName)
	cmd.Stdout = os.Stdout
	return cmd.Run()
}

// Build and Zip tar
func buildAndZipTar(tarName string) (string, error) {
	// Build tar
	Info.Println("Building tar from Docker image...")
	err := buildTarFromImage(tarName)
	if err != nil {
		Error.Println("Error building tar from image", err)
		return "", err
	}

	tarNameExt := tarName + ".tar"
	// Gzip tar
	Info.Println("Gzipping tar...")
	err = gzipTar(tarNameExt)
	if err != nil {
		Error.Println("Failure to gzip", err)
		return "", err
	}
	tarNameExt += ".gz"
	return tarNameExt, nil
}

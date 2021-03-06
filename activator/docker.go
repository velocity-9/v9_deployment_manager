package activator

import (
	"os"
	"os/exec"
	"v9_deployment_manager/log"
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
	cmd := exec.Command("pigz", tarName)
	cmd.Stdout = os.Stdout
	return cmd.Run()
}

// Build and Zip tar
func buildAndZipTar(tarName string) (string, error) {
	// Build tar
	log.Info.Println("Building tar from Docker image...")
	err := buildTarFromImage(tarName)
	if err != nil {
		log.Error.Println("Error building tar from image", err)
		return "", err
	}

	tarNameExt := tarName + ".tar"
	// Gzip tar
	log.Info.Println("Gzipping tar...")
	err = gzipTar(tarNameExt)
	if err != nil {
		log.Error.Println("Failure to gzip", err)
		return "", err
	}
	tarNameExt += ".gz"
	return tarNameExt, nil
}

func buildComponentBundle(tarName string, clonedPath string) (string, error) {
	// Build image
	log.Info.Println("Building image from Dockerfile...")
	err := buildImageFromDockerfile(tarName, clonedPath)
	if err != nil {
		log.Error.Println("Error building image from Dockerfile", err)
		return "", err
	}

	// Build and Zip Tar
	log.Info.Println("Building and zipping tar...")
	tarNameExt, err := buildAndZipTar(tarName)
	if err != nil {
		log.Error.Println("Failed to build and compress tar", err)
		return "", err
	}
	return tarNameExt, nil
}

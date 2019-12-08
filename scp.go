package main

import (
	scp "github.com/bramvdbogaerde/go-scp"
	"github.com/bramvdbogaerde/go-scp/auth"
	"golang.org/x/crypto/ssh"
	"os"
	"time"
)

func scpToWorker(workerUrl string, source string, dest string, tarName string) error {
	// Use SSH key authentication from the auth package
	// we ignore the host key in this example, please change this if you use this library
	clientConfig, err := auth.PrivateKey("ubuntu", "/home/ubuntu/.ssh/senior-design.pem", ssh.InsecureIgnoreHostKey())
	if err != nil {
		Error.Println("Error creating ssh config", err)
		return err
	}

	// Create a new SCP client
	workerUrl += ":22"
	client := scp.NewClientWithTimeout(workerUrl+":22", &clientConfig, time.Duration(100000000000))

	// Connect to the remote server
	Info.Println("Connecting to worker...")
	err = client.Connect()
	if err != nil {
		Error.Println("Couldn't establish a connection to the remote server ", err)
		return err
	}

	// Open a file
	f, err := os.Open(source)
	if err != nil {
		Error.Println("Error opening source file scp", err)
		return err
	}

	// Close client connection after the file has been copied
	defer client.Close()

	// Close the file after it has been copied
	defer f.Close()

	// Finally, copy the file over
	// Usage: CopyFile(fileReader, remotePath, permission)
	Info.Println("Copying " + tarName)
	return client.CopyFile(f, dest, "0655")
}

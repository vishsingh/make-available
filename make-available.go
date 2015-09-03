package main

/*
runit() {
  go install github.com/vishsingh/make-available && /home/work/bin/make-available
}
*/

import "fmt"
import "os"
import "os/exec"
//import "flag"

type config struct {
     host string // Remote host holding the encfs tree
     hostdir string // Path to the encfs tree on the remote host
     encfsConfig string // Path to the encfs config on the local filesystem
     group string // If run as root, users in this group will be able to access the mounted data
}

func withStdStreams(cmd *exec.Cmd) *exec.Cmd {
     newCmd := *cmd

     newCmd.Stdin = os.Stdin
     newCmd.Stdout = os.Stdout
     newCmd.Stderr = os.Stderr

     return &newCmd
}

func main() {
	cfg := getConfig()

	fmt.Printf("the host is %s\n", cfg.host)

	mountWorkspace := "/tmp/makeavailmnt"
	err := os.Mkdir(mountWorkspace, 0700)
	if err != nil {
	   panic("unable to create mount workspace")
	}
	defer os.Remove(mountWorkspace)	

	bashCmd := withStdStreams(exec.Command("/bin/bash"))
	bashCmd.Dir = mountWorkspace
	bashCmd.Run()
}

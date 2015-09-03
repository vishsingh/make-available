package main

/*
runit() {
  go install github.com/vishsingh/make-available && /home/work/bin/make-available
}
*/

import "fmt"
//import "os"
//import "os/exec"
//import "flag"

type config struct {
     host string // Remote host holding the encfs tree
     hostdir string // Path to the encfs tree on the remote host
     encfsConfig string // Path to the encfs config on the local filesystem
     group string // If run as root, users in this group will be able to access the mounted data
}

func main() {
	cfg := getConfig()

	fmt.Printf("the host is %s\n", cfg.host)

	// mountWorkspace := "/tmp/makeavailmnt"
	// err := os.Mkdir(mountWorkspace, 0700)
	// if err != nil {
	//    return
	// }
	//
	// defer os.Remove(mountWorkspace)	
	//
	// fmt.Printf("successfully created dir\n")
}

package main

/*
runit() {
  go install github.com/vishsingh/make-available && /home/work/bin/make-available
}
*/

//import "fmt"
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

// returns a function to perform the unmount
func makeImageAvailable(mountPoint string, cfg *config) (func(), error) {
     sshfsCmd := exec.Command("sshfs")

     args := []string{
     	       cfg.host + ":" + cfg.hostdir,
     	       mountPoint,
	       "-o",
	       "ro",
     }
     sshfsCmd.Args = append(sshfsCmd.Args, args...)

     sshfsCmd = withStdStreams(sshfsCmd)

     err := sshfsCmd.Run()

     if err != nil {
     	return nil, err
     }

     return func() {
	 err := exec.Command("fusermount", "-u", mountPoint).Run()
	 if err != nil {
	    panic("failed to perform sshfs unmount")
	 }
     }, nil
}

// returns a function to perform the unmount
func mountEncFs(encFsConfigPath string, encryptedDir string, mountPoint string) (func(), error) {
     encfsCmd := exec.Command("encfs", encryptedDir, mountPoint)

     encfsCmd.Env = os.Environ()
     encfsCmd.Env = append(encfsCmd.Env, "ENCFS6_CONFIG=" + encFsConfigPath)
     
     encfsCmd = withStdStreams(encfsCmd)

     if err := encfsCmd.Run(); err != nil {
     	return nil, err
     }

     return func() {
	 err := exec.Command("fusermount", "-u", mountPoint).Run()
	 if err != nil {
	    panic("failed to perform encfs unmount")
	 }
     }, nil
}

func main() {
	cfg := getConfig()

	mountWorkspace := "/tmp/makeavailmnt"
	err := os.Mkdir(mountWorkspace, 0700)
	if err != nil {
	   panic("unable to create mount workspace")
	}
	defer os.Remove(mountWorkspace)	

	mountPoint := mountWorkspace + "/mnt"
	err = os.Mkdir(mountPoint, 0755)
	if err != nil {
	   panic("unable to create mount point")
	}
	defer os.Remove(mountPoint)

	unmounter, err := makeImageAvailable(mountPoint, cfg)
	if err != nil {
	   panic("failed to make image available")
	}
	defer unmounter()

	encfsMountPoint := mountWorkspace + "/emnt"
	if err = os.Mkdir(encfsMountPoint, 0755); err != nil {
	   panic("unable to create encfs mount point")
	}
	defer os.Remove(encfsMountPoint)

	encfsUnmounter, err := mountEncFs(cfg.encfsConfig, mountPoint, encfsMountPoint)
	if err != nil {
	   panic("unable to mount encfs")
	}
	defer encfsUnmounter()	

	bashCmd := withStdStreams(exec.Command("/bin/bash"))
	bashCmd.Dir = mountWorkspace
	bashCmd.Run()
}

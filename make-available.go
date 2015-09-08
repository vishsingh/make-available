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
	host        string // Remote host holding the encfs tree
	hostdir     string // Path to the encfs tree on the remote host
	encfsConfig string // Path to the encfs config on the local filesystem
	group       string // If run as root, users in this group will be able to access the mounted data
}

func withStdStreams(cmd *exec.Cmd) *exec.Cmd {
	newCmd := *cmd

	newCmd.Stdin = os.Stdin
	newCmd.Stdout = os.Stdout
	newCmd.Stderr = os.Stderr

	return &newCmd
}

func makeCommand(name string, arg ...string) *exec.Cmd {
	return withStdStreams(exec.Command(name, arg...))
}

type commandLineError struct {
	c       exec.Cmd
	origErr error
}

func (e *commandLineError) Error() string {
	return fmt.Sprintf("command %#v with args %#v failed with error %#v", e.c.Path, e.c.Args, e.origErr.Error())
}

// same as Run(), but the error value returned will contain the entire command line that failed
func run(cmd *exec.Cmd) error {
	err := cmd.Run()
	if err != nil {
		return &commandLineError{*cmd, err}
	}
	return nil
}

// returns a function to perform the unmount
func makeImageAvailable(mountPoint string, cfg *config) (func() error, error) {
	sshfsCmd := makeCommand("sshfs")

	args := []string{
		cfg.host + ":" + cfg.hostdir,
		mountPoint,
		"-o",
		"ro",
	}
	sshfsCmd.Args = append(sshfsCmd.Args, args...)

	err := sshfsCmd.Run()

	if err != nil {
		return nil, err
	}

	return func() error {
		return run(makeCommand("fusermount", "-u", mountPoint))
	}, nil
}

// returns a function to perform the unmount
func mountEncFs(encFsConfigPath string, encryptedDir string, mountPoint string) (func() error, error) {
	encfsCmd := makeCommand("encfs", encryptedDir, mountPoint)

	encfsCmd.Env = os.Environ()
	encfsCmd.Env = append(encfsCmd.Env, "ENCFS6_CONFIG="+encFsConfigPath)

	if err := encfsCmd.Run(); err != nil {
		return nil, err
	}

	return func() error {
		return run(makeCommand("fusermount", "-u", mountPoint))
	}, nil
}

func panicUnless(thunk func() error, panicStr string) {
	err := thunk()
	if err != nil {
		panic(fmt.Sprintf("%s: %s", panicStr, err.Error()))
	}
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
	defer panicUnless(unmounter, "failed to undo makeImageAvailable")

	encfsMountPoint := mountWorkspace + "/emnt"
	if err = os.Mkdir(encfsMountPoint, 0755); err != nil {
		panic("unable to create encfs mount point")
	}
	defer os.Remove(encfsMountPoint)

	encfsUnmounter, err := mountEncFs(cfg.encfsConfig, mountPoint, encfsMountPoint)
	if err != nil {
		panic("unable to mount encfs")
	}
	defer panicUnless(encfsUnmounter, "failed to unmount encfs")

	bashCmd := makeCommand("/bin/bash")
	bashCmd.Dir = mountWorkspace
	bashCmd.Run()
}

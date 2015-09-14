package main

/*
runit() {
  go install github.com/vishsingh/make-available && "$GOPATH"/bin/make-available "$@"
}
*/

import "fmt"
import "os"
import "os/exec"
import "errors"
import "runtime"
import "os/user"
import "flag"

type mountSpec struct {
        name                string // Name of this spec; --name on the command line selects this spec
        remoteDir           string // Path to the encfs tree on the remote host
	encfsConfig         string // Path to the encfs config on the local filesystem
	checksumFile        string // Path to a file containing previous known checksums
}

type config struct {
	host                string // Remote host holding the encfs tree
	checksumTreeProgram string // Path to a program that produces the checksums of all files under a given directory
	specs               []mountSpec

	/** Controlled by flags **/

	mountRw             bool   // Mount the filesystem read-write, rather than read-only?
	doChecksum          bool   // Perform checksums of all files in the mounted filesystem?
	selectedSpec        *mountSpec
}

func (s *mountSpec) check() error {
	if len(s.name) == 0 || len(s.remoteDir) == 0 || len(s.encfsConfig) == 0 {
		return errors.New("config not fully filled out")
	}
	return nil        
}

func (c *config) check() error {
	if len(c.host) == 0 {
		return errors.New("config not fully filled out")
	}

        if c.specs == nil {
		return errors.New("config not fully filled out")
	}
	for _, spec := range c.specs {
	        if err := spec.check(); err != nil {
		        return err
		}
	}

     	s := c.selectedSpec
        if s == nil {
		return errors.New("no mountSpec was selected")
	}

	if err := s.check(); err != nil {
		return err
	}

	if c.doChecksum {
		info, err := os.Stat(s.checksumFile)
		if err != nil {
			return errors.New("unable to access checksum file")
		}
		if !info.Mode().IsRegular() {
			return errors.New("checksum file is not a regular file")
		}

		info, err = os.Stat(c.checksumTreeProgram)
		if err != nil {
			return errors.New("unable to access checksum tree program")
		}
		if info.Mode()&0111 == 0 {
			return errors.New("checksum tree program is not executable")
		}
	}

	return nil
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
	sshfsCmd := makeCommand("sshfs", cfg.host+":"+cfg.selectedSpec.remoteDir, mountPoint)

	if !cfg.mountRw {
	        sshfsCmd.Args = append(sshfsCmd.Args, "-o", "ro")
	}

	u, err := user.Current()
	if err != nil {
	        return nil, annotate(err)
	}
	sshfsCmd.Args = append(sshfsCmd.Args, "-o", "uid="+u.Uid, "-o", "gid="+u.Gid)	

	if err := sshfsCmd.Run(); err != nil {
		return nil, err
	}

	return func() error {
		return run(makeCommand("fusermount", "-u", mountPoint))
	}, nil
}

// returns a function to perform the unmount
func mountEncFs(encFsConfigPath string, encryptedDir string, mountPoint string) (func() error, error) {
	encfsCmd := makeCommand("encfs", encryptedDir, mountPoint)

	encfsCmd.Env = append(os.Environ(), "ENCFS6_CONFIG="+encFsConfigPath)

	if err := encfsCmd.Run(); err != nil {
		return nil, err
	}

	return func() error {
		return run(makeCommand("fusermount", "-u", mountPoint))
	}, nil
}

func annotate(err error) error {
	_, file, line, ok := runtime.Caller(1)

	if !ok {
		return err
	}

	return fmt.Errorf("%s, line %d: %s", file, line, err.Error())
}

func doChecksum(cfg *config, encfsMountPoint string) error {
	intermediateFile := cfg.selectedSpec.checksumFile + ".new"

	checksumRoot := encfsMountPoint

	checksumCmd := exec.Command(cfg.checksumTreeProgram, ".", cfg.selectedSpec.checksumFile)
	checksumCmd.Dir = checksumRoot

	var intermediateFileStream *os.File
	var err error
	if intermediateFileStream, err = os.Create(intermediateFile); err != nil {
		return annotate(err)
	}
	defer intermediateFileStream.Close()

	checksumCmd.Stdout = intermediateFileStream
	checksumCmd.Stderr = os.Stderr

	if err = checksumCmd.Run(); err != nil {
		return annotate(err)
	}

	intermediateFileStream.Close()

	if err = os.Rename(intermediateFile, cfg.selectedSpec.checksumFile); err != nil {
		return annotate(err)
	}

	fmt.Printf("Checksum file updated at %s.\n", cfg.selectedSpec.checksumFile)
	return nil
}

func panicUnless(thunk func() error, panicStr string) {
	if err := thunk(); err != nil {
		panic(fmt.Sprintf("%s: %s", panicStr, err.Error()))
	}
}

func main() {
	cfg := getConfig()

	rwFlag           := flag.Bool("rw", false, "Mount the filesystem read-write, rather than read-only")
	noChecksumFlag   := flag.Bool("no-checksum", false, "Do not perform a checksum after user is done with the filesystem")

	specNameFlags := make([]bool, len(cfg.specs))
	for i, spec := range cfg.specs {
		flag.BoolVar(&(specNameFlags[i]), spec.name, false, fmt.Sprintf("Mount the '%s' filesystem", spec.name))
	}

	flag.Parse()

	cfg.mountRw = *rwFlag
	cfg.doChecksum = *rwFlag && !*noChecksumFlag

	for i, specFlag := range specNameFlags {
	        if specFlag {
		   	cfg.selectedSpec = &(cfg.specs[i])
		        break
		}
	}

	if err := cfg.check(); err != nil {
		panic(err)
	}

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

	encfsUnmounter, err := mountEncFs(cfg.selectedSpec.encfsConfig, mountPoint, encfsMountPoint)
	if err != nil {
		panic("unable to mount encfs")
	}
	defer panicUnless(encfsUnmounter, "failed to unmount encfs")

	bashCmd := makeCommand("/bin/bash")
	bashCmd.Dir = mountWorkspace
	bashCmd.Run()

	if cfg.doChecksum {
		// todo: remount as read-only if necessary

		if err := doChecksum(cfg, encfsMountPoint); err != nil {
			panic(err)
		}
	}
}

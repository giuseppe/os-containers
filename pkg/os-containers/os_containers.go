package oscontainers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// #include <unistd.h>
import "C"

const (
	Running = iota
	Stopped
	Failed
)

type Container struct {
	Name                   string                 `json:"-"`
	OstreeCommit           string                 `json:"ostree-commit"`
	HasContainerService    bool                   `json:"has-container-service"`
	Revision               string                 `json:"revision"`
	Remote                 string                 `json:"remote"`
	Image                  string                 `json:"image"`
	Created                int64                  `json:"created"`
	Runtime                string                 `json:"runtime"`
	InstalledFiles         []string               `json:"installed-files"`
	InstalledFilesTemplate []string               `json:"installed-files-template"`
	InstalledFilesChecksum map[string]string      `json:"installed-files-checksum"`
	RenameInstalledFiles   map[string]string      `json:"rename-installed-files"`
	Values                 map[string]interface{} `json:"values"`

	// Old info files have the map[string]interface{}, keep
	// also the string->string version to avoid converting back
	// and forth.
	values map[string]string
}

func GetContainerStatusString(s int) string {
	status := []string{"Running", "Stopped", "Failed"}
	return status[s]
}

func (c *Container) ContainerStatus() (int, error) {
	if _, err := systemctlCommand("is-active", c.Name, false, true); err == nil {
		return Running, nil
	}
	if _, err := systemctlCommand("is-failed", c.Name, false, true); err == nil {
		return Failed, nil
	}
	return Stopped, nil
}

func (c *Container) WriteToFile(path string) error {
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}

	var out bytes.Buffer
	json.Indent(&out, b, "", "    ")

	outFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0700)
	if err != nil {
		return err
	}
	defer outFile.Close()

	_, err = out.WriteTo(outFile)
	return err
}

func ReadContainer(checkouts string, name string, deployment *int) (*Container, error) {
	var container Container

	container.Name = name

	subdir := name
	if deployment != nil {
		subdir = fmt.Sprintf("%s.%d", subdir, deployment)
	}
	info, err := ioutil.ReadFile(filepath.Join(checkouts, name, "info"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("cannot find container %s", name)
		}
		return nil, err
	}

	if err := json.Unmarshal(info, &container); err != nil {
		return nil, err
	}
	return &container, nil
}

func systemctlCommand(cmd string, name string, now bool, quiet bool) ([]byte, error) {
	var args []string
	if os.Geteuid() != 0 {
		args = append(args, "--user")
	}
	if now {
		args = append(args, "--now")
	}
	args = append(args, cmd)
	if name != "" {
		args = append(args, name)
	}
	if !quiet {
		log.Println(fmt.Sprintf("systemctl %s", strings.Join(args, " ")))
	}
	c := exec.Command("systemctl", args...)
	return c.CombinedOutput()
}

func systemdTmpFilesCommand(cmd string, name string, quiet bool) ([]byte, error) {
	var args []string
	if os.Geteuid() != 0 {
		args = append(args, "--user")
	}
	args = append(args, cmd)
	if name != "" {
		args = append(args, name)
	}
	if !quiet {
		log.Println(fmt.Sprintf("systemd-tmpfiles %s", strings.Join(args, " ")))
	}
	c := exec.Command("systemd-tmpfiles", args...)
	return c.CombinedOutput()
}

func GetContainers(all bool) ([]Container, error) {
	checkouts := getCheckoutsDirectory()
	files, err := ioutil.ReadDir(checkouts)
	if err != nil {
		return nil, err
	}
	containers := []Container{}
	for _, f := range files {
		if f.Mode()&os.ModeSymlink != 0 {
			c, err := ReadContainer(checkouts, f.Name(), nil)
			if err != nil {
				return nil, err
			}
			containers = append(containers, *c)
		}
	}
	return containers, nil
}

func getCheckoutsDirectory() string {
	e := os.Getenv("OS_CONTAINERS_CHECKOUT_PATH")
	if e != "" {
		return e
	}

	if os.Geteuid() == 0 {
		return "/var/lib/containers/atomic"
	} else {
		xdgDataDir := os.Getenv("XDG_DATA_DIR")
		if xdgDataDir != "" {
			return filepath.Join(xdgDataDir, "containers/atomic")
		}
		return filepath.Join(os.Getenv("HOME"), ".containers/atomic")
	}
}

func getRuntime() string {
	runtime := os.Getenv("RUNTIME")
	if runtime != "" {
		return runtime
	}
	return "/usr/bin/runc"
}

func deleteCheckouts(name string, checkouts string) error {
	i := 0
	var err error
	for {
		checkout := filepath.Join(checkouts, fmt.Sprintf("%s.%d", name, i))
		if _, err := os.Stat(checkout); err != nil {
			break
		}
		err2 := os.RemoveAll(checkout)
		if err2 != nil {
			err = err2
		}
		i = i + 1
	}
	return err
}

func destroyActiveCheckout(c *Container, checkouts string) error {
	from := filepath.Join(checkouts, c.Name)
	fi, err := os.Lstat(from)
	if err != nil {
		return err
	}
	if (fi.Mode() & os.ModeSymlink) != os.ModeSymlink {
		return fmt.Errorf("%s is not a symbolic link", from)
	}
	if c.HasContainerService {
		systemctlCommand("disable", c.Name, true, false)
		filename := fmt.Sprintf("%s.service", c.Name)
		unitFile := filepath.Join(getSystemdDestination(), filename)
		os.Remove(unitFile)

		_, err := os.Stat(filepath.Join(from, "rootfs/exports/tmpfiles.template"))
		if err == nil {
			filename := fmt.Sprintf("%s.conf", c.Name)
			tmpFiles := filepath.Join(getSystemdTmpFilesDestination(), filename)
			systemdTmpFilesCommand("--delete", tmpFiles, false)
			os.Remove(tmpFiles)
		}
	}
	for _, f := range c.InstalledFiles {
		oldChecksum := c.InstalledFilesChecksum[f]
		newChecksum, err := getFileChecksum(f)
		if err != nil {
			continue
		}
		/* The file was not modified since its installation.  */
		if newChecksum != oldChecksum {
			log.Println(fmt.Sprintf("file %s was modified.  Skip.", f))
		} else {
			err = os.Remove(f)
			if err != nil {
				log.Println(fmt.Sprintf("file %s deleted", f))
			} else {
				log.Println(fmt.Sprintf("could not delete %s: %v", err))
			}
		}
	}
	/* All is cleaned up, delete the symlink.  */
	os.Remove(from)
	return nil
}

func RunCommand(container string, command []string) error {
	checkouts := getCheckoutsDirectory()
	c, err := ReadContainer(checkouts, container, nil)
	if err != nil {
		return err
	}

	s, err := c.ContainerStatus()
	if err != nil {
		return err
	}

	if s != Running {
		return fmt.Errorf("%s is not running", container)
	}

	var args []string
	if C.isatty(1) != 0 {
		args = append([]string{"exec", "-t", container}, command...)
	} else {
		args = append([]string{"exec", container}, command...)
	}
	cmd := exec.Command(c.Runtime, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

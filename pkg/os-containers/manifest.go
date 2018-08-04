package oscontainers

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/mrunalp/fileutils"
	"github.com/opencontainers/go-digest"
	"github.com/satori/go.uuid"
)

type descriptor struct {
	Size   int64         `json:"size"`
	Digest digest.Digest `json:"digest"`
}

type fsLayersSchema1 struct {
	BlobSum digest.Digest `json:"blobSum"`
}

type manifestSchema struct {
	LayersDescriptors []descriptor      `json:"layers"`
	FSLayers          []fsLayersSchema1 `json:"fsLayers"`
}

func getLayers(manifestBlob []byte) ([]string, error) {
	var ret []string
	var schema manifestSchema
	if err := json.Unmarshal(manifestBlob, &schema); err != nil {
		return nil, err
	}
	for _, layer := range schema.LayersDescriptors {
		hash := layer.Digest.Hex()
		ret = append(ret, hash)
	}
	for _, layer := range schema.FSLayers {
		hash := layer.BlobSum.Hex()
		// In reverse order
		ret = append([]string{hash}, ret...)
	}
	return ret, nil
}

const defaultService string = `
[Unit]
Description=$NAME

[Service]
ExecStartPre=$EXEC_STARTPRE
ExecStart=$EXEC_START
ExecStop=$EXEC_STOP
ExecStopPost=$EXEC_STOPPOST
Restart=on-failure
WorkingDirectory=$DESTDIR
PIDFile=$PIDFILE

[Install]
WantedBy=multi-user.target
`

func copyFile(src, dest string) error {
	err := os.MkdirAll(path.Dir(dest), 0755)
	if err != nil {
		return err
	}
	return fileutils.CopyFile(src, dest)
}

func copyFileAndRelabel(ctx *SELinuxCtx, src, dest string) error {
	if err := copyFile(src, dest); err != nil {
		return err
	}
	return ctx.Label(dest)
}

func checkConfigHasPidfile(file string) (bool, error) {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		if os.IsNotExist(err) {
			return true, nil
		}
		return false, err
	}
	return strings.Contains(string(data), "PIDFILE"), nil
}

func setSystemdStartup(runtime, srcFile, name string, values map[string]string) error {
	hasPidFile, err := checkConfigHasPidfile(srcFile)
	if err != nil {
		return err
	}
	var start, stop, stoppost, prestart string
	if hasPidFile {
		if _, found := values["PIDFILE"]; !found {
			values["PIDFILE"] = filepath.Join(values["RUN_DIRECTORY"], fmt.Sprintf("container-%s.pid", name))
		}
		pidfile := values["PIDFILE"]
		start = fmt.Sprintf("%s --systemd-cgroup run -d --pidfile %s '%s'", runtime, pidfile, name)
		stoppost = fmt.Sprintf("%s delete '%s'", runtime, name)
	} else {
		start = fmt.Sprintf("%s --systemd-cgroup run '%s'", runtime, name)
		stop = fmt.Sprintf("%s kill '%s'", runtime, name)
	}
	values["EXEC_START"] = start
	values["EXEC_STOP"] = stop
	values["EXEC_STARTPRE"] = prestart
	values["EXEC_STOPPOST"] = stoppost
	return nil
}

func amendValues(name, image, imageID string, values map[string]string) error {
	root := os.Geteuid() == 0
	if _, found := values["RUN_DIRECTORY"]; !found {
		if root {
			values["RUN_DIRECTORY"] = "/run"
		} else {
			runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
			if runtimeDir == "" {
				runtimeDir = fmt.Sprintf("/run/user/%d", os.Geteuid())
			}
			values["RUN_DIRECTORY"] = runtimeDir
		}
	}

	if _, found := values["CONF_DIRECTORY"]; !found {
		if root {
			values["CONF_DIRECTORY"] = "/etc"
		} else {
			values["RUN_DIRECTORY"] = filepath.Join(os.Getenv("HOME"), ".config")
		}
	}

	if _, found := values["STATE_DIRECTORY"]; !found {
		if root {
			values["STATE_DIRECTORY"] = "/var/lib"
		} else {
			values["STATE_DIRECTORY"] = filepath.Join(os.Getenv("HOME"), ".data")
		}
	}
	if _, found := values["UUID"]; !found {
		values["UUID"] = uuid.Must(uuid.NewV4()).String()

	}

	values["HOST_UID"] = fmt.Sprintf("%d", os.Geteuid())
	values["HOST_GID"] = fmt.Sprintf("%d", os.Getegid())
	values["IMAGE_NAME"] = image
	values["IMAGE_ID"] = imageID

	return nil
}

func generateDefaultConfigFile(runtime, destConfig string) error {
	var cmd *exec.Cmd
	if os.Geteuid() == 0 {
		cmd = exec.Command(runtime, "spec")
	} else {
		cmd = exec.Command(runtime, "spec", "--rootless")
	}
	cmd.Dir = path.Dir(destConfig)
	return cmd.Run()
}

func checkoutContainerTo(branch string, repo *OSTreeRepo, checkouts string, set map[string]string, name, image, imageID string, checkoutNumber int) (*Container, error) {
	runtimePath := getRuntime()
	found, manifest, err := repo.readMetadata(branch, "docker.manifest")
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("cannot find manifest for %s", branch)
	}

	destDir := filepath.Join(checkouts, fmt.Sprintf("%s.%d", name, checkoutNumber))

	checkout := filepath.Join(destDir, "rootfs")

	layers, err := getLayers([]byte(manifest))
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(checkout, 0700); err != nil {
		return nil, err
	}

	dir, err := os.Open(checkout)
	if err != nil {
		return nil, err
	}
	defer dir.Close()

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	for _, l := range layers {
		err = repo.unionCheckout(l, int(dir.Fd()), checkout)
		if err != nil {
			return nil, err
		}
	}

	var containerManifest *ContainerManifest
	manifestFile := filepath.Join(checkout, "exports/manifest.json")
	if _, err := os.Stat(manifestFile); err == nil {
		containerManifest, err = ReadContainerManifest(manifestFile)
		if err != nil {
			return nil, err
		}
	}

	srcConfig := filepath.Join(checkout, "exports/config.json.template")
	destConfig := filepath.Join(destDir, "config.json")

	srcServiceConfig := filepath.Join(checkout, "exports/service.template")
	destServiceConfig := filepath.Join(destDir, fmt.Sprintf("%s.service", name))

	srcTempFiles := filepath.Join(checkout, "exports/tmpfiles.template")
	destTempFiles := filepath.Join(destDir, fmt.Sprintf("tmpfiles-%s.conf", name))

	values := make(map[string]string)

	if containerManifest != nil {
		for k, v := range containerManifest.DefaultValues {
			values[k] = v
		}
	}

	for k, v := range set {
		values[k] = v
	}

	err = amendValues(name, image, imageID, values)
	if err != nil {
		return nil, err
	}

	err = setSystemdStartup(runtimePath, srcServiceConfig, name, values)
	if err != nil {
		return nil, err
	}

	values["NAME"] = name
	values["DESTDIR"] = destDir

	if containerManifest != nil {
		newRenameFiles := make(map[string]string)
		for k, v := range containerManifest.RenameFiles {
			nv, err := TemplateReplaceMemory(v, values)
			if err != nil {
				return nil, err
			}
			newRenameFiles[k] = nv
		}
		containerManifest.RenameFiles = newRenameFiles
	}

	if _, err := os.Stat(srcConfig); err != nil && os.IsNotExist(err) {
		err = generateDefaultConfigFile(runtimePath, destConfig)
		if err != nil {
			return nil, err
		}
	} else {
		err = TemplateWithDefaultGenerate(srcConfig, destConfig, "", values)
		if err != nil {
			return nil, err
		}
	}

	err = TemplateWithDefaultGenerate(srcServiceConfig, destServiceConfig, defaultService, values)
	if err != nil {
		return nil, err
	}

	var hasTempFiles bool
	if _, err = os.Stat(srcTempFiles); err == nil {
		hasTempFiles = true
	}
	if hasTempFiles {
		err = TemplateWithDefaultGenerate(srcTempFiles, destTempFiles, "", values)
		if err != nil {
			return nil, err
		}
	}

	var renameFiles map[string]string
	var installedFilesTemplate []string
	valuesForContainer := make(map[string]interface{})
	for k, v := range values {
		valuesForContainer[k] = v
	}
	if containerManifest != nil {
		renameFiles = containerManifest.RenameFiles
		installedFilesTemplate = containerManifest.InstalledFilesTemplate
	}

	c := &Container{
		Name:                   name,
		OstreeCommit:           "0",
		HasContainerService:    containerManifest == nil || !containerManifest.NoContainerService,
		Revision:               imageID,
		Image:                  image,
		Created:                time.Now().Unix(),
		Runtime:                runtimePath,
		InstalledFiles:         []string{},
		InstalledFilesTemplate: installedFilesTemplate,
		RenameInstalledFiles:   renameFiles,
		Values:                 valuesForContainer,
		values:                 values,
	}

	return c, nil
}

func makeDeploymentActive(container *Container, checkouts string, name string, start bool, checkoutNumber int) error {
	destDir := filepath.Join(checkouts, fmt.Sprintf("%s.%d", name, checkoutNumber))
	checkout := filepath.Join(destDir, "rootfs")

	destServiceConfig := filepath.Join(destDir, fmt.Sprintf("%s.service", name))

	srcTempFiles := filepath.Join(checkout, "exports/tmpfiles.template")
	destTempFiles := filepath.Join(destDir, fmt.Sprintf("tmpfiles-%s.conf", name))

	if os.Geteuid() == 0 {
		hostFS := filepath.Join(destDir, "rootfs/exports/hostfs")
		copiedFiles, err := copyFilesToHost(hostFS, "/", container)
		if err != nil {
			return err
		}
		container.InstalledFiles = copiedFiles.Copied
		container.InstalledFilesChecksum = copiedFiles.Checksum
		infoFile := filepath.Join(destDir, "info")
		err = container.WriteToFile(infoFile)
		if err != nil {
			return err
		}
	}

	destSymlink := filepath.Join(checkouts, name)

	if !container.HasContainerService {
		return os.Symlink(destDir, destSymlink)
	}

	err := copyFile(destServiceConfig, filepath.Join(getSystemdDestination(), path.Base(destServiceConfig)))
	if err != nil {
		return err
	}

	var tmpFiles string
	var hasTempFiles bool
	if os.Stat(srcTempFiles); err != nil {
		hasTempFiles = true
	}
	if hasTempFiles {
		tmpFiles = filepath.Join(getSystemdTmpFilesDestination(), path.Base(destTempFiles))
		err := copyFile(destTempFiles, tmpFiles)
		if err != nil {
			return err
		}
	}

	err = os.Symlink(destDir, destSymlink)
	if err != nil {
		return err
	}

	_, err = systemctlCommand("daemon-reload", "", false, false)
	if err != nil {
		return err
	}

	_, err = systemctlCommand("enable", name, start, false)
	if err != nil {
		return err
	}

	if hasTempFiles {
		_, err := systemdTmpFilesCommand("--create", tmpFiles, false)
		if err != nil {
			return err
		}
	}

	return nil
}

func getSystemdTmpFilesDestination() string {
	if os.Geteuid() == 0 {
		return "/etc/tmpfiles.d"
	}
	xdgDataDir := os.Getenv("XDG_DATA_DIR")
	if xdgDataDir != "" {
		return filepath.Join(xdgDataDir, "containers/tmpfiles")
	}
	return filepath.Join(os.Getenv("HOME"), ".containers/tmpfiles")
}

func getSystemdDestination() string {
	if os.Geteuid() == 0 {
		return "/etc/systemd/system"
	}
	return filepath.Join(os.Getenv("HOME"), ".config/systemd/user")
}

type CopiedFiles struct {
	Copied   []string
	Checksum map[string]string
}

func getFileChecksum(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func copyFilesToHost(from string, to string, container *Container) (*CopiedFiles, error) {
	ret := &CopiedFiles{
		Copied:   []string{},
		Checksum: make(map[string]string),
	}
	if _, err := os.Stat(from); err != nil && os.IsNotExist(err) {
		return ret, nil
	}
	selinuxCtx, err := makeSELinuxCtx()
	if err != nil {
		return ret, err
	}
	defer selinuxCtx.Close()

	templates := make(map[string]string)
	for _, v := range container.InstalledFilesTemplate {
		templates[v] = v
	}

	err = filepath.Walk(from, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(from, path)
		if err != nil {
			return err
		}
		dest := filepath.Join(to, rel)
		canonicalName := fmt.Sprintf("/%s", rel)
		if r, ok := container.RenameInstalledFiles[canonicalName]; ok {
			dest = r
		}

		if _, err := os.Stat(dest); err == nil {
			return nil
		}

		if _, ok := templates[canonicalName]; ok {
			if err := TemplateWithDefaultGenerate(path, dest, "", container.values); err != nil {
				return err
			}
		} else {
			if err := copyFileAndRelabel(selinuxCtx, path, dest); err != nil {
				return err
			}
		}

		checksum, err := getFileChecksum(dest)
		if err != nil {
			return err
		}

		ret.Copied = append(ret.Copied, dest)
		ret.Checksum[dest] = checksum
		log.Println(fmt.Sprintf("copied %s", dest))

		return nil
	})
	return ret, err

}

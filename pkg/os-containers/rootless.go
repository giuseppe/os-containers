package oscontainers

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"

	"github.com/containers/storage/pkg/idtools"
	rspec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/pkg/errors"
)

func makeOCIConfigurationRootless(path string) error {
	content, err := ioutil.ReadFile(path)
	if err != nil {
		return errors.Wrapf(err, "cannot open file %s", path)
	}

	var spec rspec.Spec
	if err := json.Unmarshal(content, &spec); err != nil {
		return errors.Wrapf(err, "unmarshal container %s conf file", path)
	}

	spec.Linux.Resources = nil

	g := generate.NewFromSpec(&spec)
	g.SetProcessTerminal(false)
	g.SetRootReadonly(true)

	hasUserNS := false
	for _, ns := range g.Config.Linux.Namespaces {
		if ns.Type == rspec.UserNamespace {
			hasUserNS = true
			break
		}
	}

	if !hasUserNS {
		if err := g.AddOrReplaceLinuxNamespace(rspec.UserNamespace, ""); err != nil {
			return err
		}

		username := os.Getenv("USER")
		if username == "" {
			user, err := user.LookupId(fmt.Sprintf("%d", os.Geteuid()))
			if err != nil {
				return errors.Wrapf(err, "could not find user by UID nor USER env was set")
			}
			username = user.Username
		}
		mappings, err := idtools.NewIDMappings(username, username)
		if err != nil {
			return err
		}
		g.ClearLinuxUIDMappings()
		g.AddLinuxUIDMapping(uint32(os.Geteuid()), 0, 1)
		for _, i := range mappings.UIDs() {
			g.AddLinuxUIDMapping(uint32(i.HostID), uint32(i.ContainerID+1), uint32(i.Size))
		}
		g.ClearLinuxGIDMappings()
		g.AddLinuxGIDMapping(uint32(os.Getegid()), 0, 1)
		for _, i := range mappings.GIDs() {
			g.AddLinuxGIDMapping(uint32(i.HostID), uint32(i.ContainerID+1), uint32(i.Size))
		}
	}

	g.RemoveMount("/dev/pts")
	devPts := rspec.Mount{
		Destination: "/dev/pts",
		Type:        "devpts",
		Source:      "devpts",
		Options:     []string{"nosuid", "noexec", "newinstance", "ptmxmode=0666", "mode=0620"},
	}
	g.AddMount(devPts)

	g.SetLinuxCgroupsPath("")

	g.SaveToFile(path, generate.ExportOptions{})
	return nil
}

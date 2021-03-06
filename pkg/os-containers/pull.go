package oscontainers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/containers/image/copy"
	"github.com/containers/image/docker/reference"
	"github.com/containers/image/docker/tarfile"
	"github.com/containers/image/ostree"
	"github.com/containers/image/signature"
	"github.com/containers/image/transports/alltransports"
	"github.com/containers/image/types"
	"github.com/ostreedev/ostree-go/pkg/otbuiltin"
	"github.com/pkg/errors"
)

func getStoragePath() string {
	if os.Geteuid() != 0 {
		dataDir := os.Getenv("XDG_DATA_HOME")
		if dataDir == "" {
			home := os.Getenv("HOME")
			resolvedHome, err := filepath.EvalSymlinks(home)
			if err == nil {
				home = resolvedHome
			}
			dataDir = filepath.Join(home, ".local", "share")
		}
		return filepath.Join(dataDir, "containers/atomic/.storage")
	}
	return "/var/lib/containers/atomic/.storage"
}

func getOSTreeRepo() string {
	repo := os.Getenv("OSTREE_REPO")
	if repo != "" {
		return repo
	}
	return filepath.Join(getStoragePath(), "repo")
}

func ensureRepoExists(repoLocation string) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	_, err := os.Stat(repoLocation)
	if err != nil && !os.IsNotExist(err) {
		return err
	} else if err != nil {
		if err := os.MkdirAll(repoLocation, 0700); err != nil {
			return errors.Wrap(err, "could not create OSTree repository directory: %v")
		}

		init := otbuiltin.NewInitOptions()
		if os.Geteuid() != 0 {
			init.Mode = "bare-user"
		}

		if _, err := otbuiltin.Init(repoLocation, init); err != nil {
			return errors.Wrap(err, "could not create OSTree repository")
		}
	}
	return nil
}

func getOSTreeReference(image types.ImageReference, repo string) (types.ImageReference, error) {
	ref := image.DockerReference()
	if ref == nil {
		src, err := tarfile.NewSourceFromFile(image.StringWithinTransport())
		if err == nil {
			defer src.Close()
			manifest, err := src.LoadTarManifest()
			if err != nil {
				return nil, err
			}
			for _, i := range(manifest) {
				if i.RepoTags != nil {
					return ostree.NewReference(i.RepoTags[0], repo)
				}
			}
		}

	}

	if digested, ok := ref.(reference.Digested); ok {
		n := fmt.Sprintf("%s@%s", ref.Name(), digested.Digest())
		return ostree.NewReference(n, repo)
	} else if tagged, ok := ref.(reference.Tagged); ok {
		n := fmt.Sprintf("%s:%s", ref.Name(), tagged.Tag())
		return ostree.NewReference(n, repo)
	}
	return ostree.NewReference(ref.Name(), repo)
}

func PullImage(insecure bool, image string) error {
	repo := getOSTreeRepo()

	if err := ensureRepoExists(repo); err != nil {
		return err
	}

	policy, err := signature.DefaultPolicy(nil)
	if err != nil {
		return err
	}

	policyContext, err := signature.NewPolicyContext(policy)
	if err != nil {
		return err
	}

	srcRef, err := alltransports.ParseImageName(image)
	if err != nil {
		return fmt.Errorf("Invalid source name %s: %v", image, err)
	}
	destRef, err := getOSTreeReference(srcRef, repo)
	if err != nil {
		return fmt.Errorf("Invalid destination name %s: %v", image, err)
	}

	destinationCtx := &types.SystemContext{
		DockerInsecureSkipTLSVerify: insecure,
	}

	return copy.Image(context.Background(), policyContext, destRef, srcRef, &copy.Options{
		ReportWriter:   os.Stdout,
		DestinationCtx: destinationCtx,
	})
}

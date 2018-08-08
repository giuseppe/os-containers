package oscontainers

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/containers/image/transports/alltransports"
	"github.com/containers/image/types"
	"github.com/pkg/errors"
)

var layerRegex = regexp.MustCompile(`^[A-Fa-f0-9]{64}$`)

type Image struct {
	Name         string
	OSTreeBranch string
	OSTreeCommit string
	Intermediate bool
	ImageID      string
	Size         uint64
}

func GetImages(all bool) ([]Image, error) {
	repoPath := getOSTreeRepo()

	if _, err := os.Stat(repoPath); err != nil {
		return nil, err
	}
	repo, err := openRepo(repoPath)
	if err != nil {
		return nil, err
	}
	return getImages(repo, all)
}

func computeImageSize(repo *OSTreeRepo, branch string, sizes map[string]uint64) (uint64, error) {
	found, manifest, err := repo.readMetadata(branch, "docker.manifest")
	if err != nil {
		return 0, err
	}
	if !found {
		return 0, nil
	}
	layers, err := getLayers([]byte(manifest))
	if err != nil {
		return 0, err
	}
	var ret uint64
	for _, l := range layers {
		ret = ret + sizes[l]
	}
	return ret, nil
}

func getImages(repo *OSTreeRepo, all bool) ([]Image, error) {
	branches, err := repo.getBranches(ostreePrefix)
	if err != nil {
		return nil, err
	}

	ret := []Image{}
	sizes := make(map[string]uint64)

	for k, _ := range branches {
		branch := fmt.Sprintf("%s/%s", ostreePrefix, k)
		_, size, err := repo.readMetadata(branch, "docker.uncompressed_size")
		if err == nil {
			s, err := strconv.ParseUint(size, 10, 64)
			if err == nil {
				sizes[k] = s
			}
		}
	}

	for k, v := range branches {
		intermediate := layerRegex.MatchString(k)
		if !all && intermediate {
			continue
		}
		branch := fmt.Sprintf("%s/%s", ostreePrefix, k)

		found, imageID, err := repo.readMetadata(branch, "docker.digest")
		if err != nil || !found {
			imageID = k
		}

		name := decodeOStreeRef(k)

		var size uint64
		if intermediate {
			name = ""
			size = sizes[k]
		} else {
			size, _ = computeImageSize(repo, branch, sizes)
		}

		imageID = strings.TrimPrefix(imageID, "sha256:")

		i := Image{
			Name:         name,
			OSTreeBranch: branch,
			OSTreeCommit: v,
			Intermediate: intermediate,
			ImageID:      imageID,
			Size:         size,
		}
		ret = append(ret, i)
	}

	return ret, nil
}

func DeleteImage(name string) error {
	srcRef, err := parseImageName(name)
	if err != nil {
		return err
	}

	dockerRef := srcRef.DockerReference()

	branch := fmt.Sprintf("%s/%s", ostreePrefix, encodeOStreeRef(dockerRef.String()))

	repoPath := getOSTreeRepo()

	if _, err := os.Stat(repoPath); err != nil {
		return errors.Wrapf(err, "stat %s", repoPath)
	}
	repo, err := openRepo(repoPath)
	if err != nil {
		return err
	}
	return repo.deleteBranch(branch)
}

func PruneImages() error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	repoPath := getOSTreeRepo()

	if _, err := os.Stat(repoPath); err != nil {
		return errors.Wrapf(err, "stat %s", repoPath)
	}
	repo, err := openRepo(repoPath)
	if err != nil {
		return err
	}

	images, err := getImages(repo, true)
	if err != nil {
		return err
	}
	seen := make(map[string]string)
	for _, i := range images {
		if !i.Intermediate {
			found, manifest, err := repo.readMetadata(i.OSTreeBranch, "docker.manifest")
			if err != nil {
				return err
			}
			if !found {
				return fmt.Errorf("cannot find manifest for %s", i.Name)
			}

			layers, err := getLayers([]byte(manifest))
			if err != nil {
				return err
			}
			for _, l := range layers {
				seen[l] = l
			}
		}
	}

	for _, i := range images {
		if i.Intermediate {
			_, ok := seen[i.Name]
			if ok {
				log.Printf("layer %s: keep", i.Name)
			} else {
				if err := repo.deleteBranch(i.OSTreeBranch); err != nil {
					return err
				}
				log.Printf("layer %s: delete", i.Name)
			}
		}
	}
	size, err := repo.prune()
	if err != nil {
		return err
	}
	log.Printf("pruned %v bytes", size)
	return nil
}

func parseImageName(image string) (types.ImageReference, error) {
	srcRef, err := alltransports.ParseImageName(image)
	if err != nil {
		return alltransports.ParseImageName(fmt.Sprintf("docker://%s", image))
	}
	return srcRef, err
}

func TagImage(src, dest string) error {
	srcRef, err := parseImageName(src)
	if err != nil {
		return err
	}
	dockerRef := srcRef.DockerReference()
	srcBranch := fmt.Sprintf("%s/%s", ostreePrefix, encodeOStreeRef(dockerRef.String()))

	destRef, err := parseImageName(dest)
	if err != nil {
		return err
	}
	dockerRef = destRef.DockerReference()
	destBranch := fmt.Sprintf("%s/%s", ostreePrefix, encodeOStreeRef(dockerRef.String()))

	repoPath := getOSTreeRepo()
	if _, err := os.Stat(repoPath); err != nil {
		return errors.Wrapf(err, "stat %s", repoPath)
	}
	repo, err := openRepo(repoPath)
	if err != nil {
		return err
	}

	commit, err := repo.resolveCommit(srcBranch)
	if err != nil {
		return err
	}

	return repo.setBranch(destBranch, commit)
}

package oscontainers

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"runtime"

	"github.com/containers/image/transports/alltransports"
	"github.com/containers/image/types"
)

var layerRegex = regexp.MustCompile(`^[A-Fa-f0-9]{64}$`)

type Image struct {
	Name         string
	OSTreeBranch string
	OSTreeCommit string
	Intermediate bool
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

func getImages(repo *OSTreeRepo, all bool) ([]Image, error) {
	branches, err := repo.getBranches(ostreePrefix)
	if err != nil {
		return nil, err
	}

	ret := []Image{}
	for k, v := range branches {
		intermediate := layerRegex.MatchString(k)
		if !all && intermediate {
			continue
		}
		i := Image{
			Name:         decodeOStreeRef(k),
			OSTreeBranch: fmt.Sprintf("%s/%s", ostreePrefix, k),
			OSTreeCommit: v,
			Intermediate: intermediate,
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
		return err
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
		return err
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

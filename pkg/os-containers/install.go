package oscontainers

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/containers/image/docker/reference"
	"github.com/containers/image/transports/alltransports"
)

func getDefaultContainerName(ref reference.Named) string {
	parts := strings.Split(reference.Path(ref), "/")
	name := parts[len(parts)-1]

	if tagged, ok := ref.(reference.Tagged); ok {
		tag := tagged.Tag()
		if tag != "latest" {
			return fmt.Sprintf("%s-%s", name, tag)
		}
	}
	return name
}

func InstallContainer(name, image string, set map[string]string) error {
	repoPath := getOSTreeRepo()

	if _, err := os.Stat(repoPath); err != nil {
		return err
	}

	srcRef, err := alltransports.ParseImageName(fmt.Sprintf("docker://%s", image))
	if err != nil {
		return err
	}

	dockerRef := srcRef.DockerReference()

	if name == "" {
		name = getDefaultContainerName(dockerRef)
	}

	checkouts := getCheckoutsDirectory()

	checkout := filepath.Join(checkouts, name)
	if _, err := os.Stat(checkout); err == nil {
		return fmt.Errorf("the container %s already exists", name)
	}

	repo, err := openRepo(repoPath)
	if err != nil {
		return err
	}

	branch := fmt.Sprintf("%s/%s", ostreePrefix, encodeOStreeRef(dockerRef.String()))
	hasBranch, err := repo.hasBranch(branch)
	if err != nil {
		return err
	}

	if !hasBranch {
		if err := PullImage(image); err != nil {
			return err
		}
	}

	_, imageID, err := repo.readMetadata(branch, "docker.digest")
	if err != nil {
		return err
	}

	imageID = strings.TrimPrefix(imageID, "sha256:")

	container, err := checkoutContainerTo(branch, repo, checkouts, set, name, image, imageID, 0)
	if err != nil {
		return err
	}

	return makeDeploymentActive(container, checkouts, name, false, 0)
}

func UninstallContainer(name string) error {
	checkouts := getCheckoutsDirectory()

	ctr, err := ReadContainer(checkouts, name, nil)
	if err != nil {
		deleteCheckouts(name, checkouts)
		return err
	}

	err = destroyActiveCheckout(ctr, checkouts)
	if err != nil {
		deleteCheckouts(name, checkouts)
		return err
	}

	return deleteCheckouts(name, checkouts)
}

func getCurrentRevision(checkout string) (int, error) {
	target, err := filepath.EvalSymlinks(checkout)
	if err != nil {
		return -1, err
	}

	ind := strings.LastIndex(target, ".")
	if ind < 1 {
		return -1, fmt.Errorf("invalid checkout name %s", target)
	}
	return strconv.Atoi(target[ind+1:])
}

func UpdateContainer(name string, set map[string]string, rebase string) error {
	repoPath := getOSTreeRepo()

	checkouts := getCheckoutsDirectory()

	ctr, err := ReadContainer(checkouts, name, nil)
	if err != nil {
		return err
	}

	checkout := filepath.Join(checkouts, name)

	rev, err := getCurrentRevision(checkout)
	if err != nil {
		return err
	}

	nextRevision := ^rev & 0x1

	repo, err := openRepo(repoPath)
	if err != nil {
		return err
	}

	image := ctr.Image
	if rebase != "" {
		image = rebase
	}

	srcRef, err := alltransports.ParseImageName(fmt.Sprintf("docker://%s", image))
	if err != nil {
		return err
	}

	dockerRef := srcRef.DockerReference()

	branch := fmt.Sprintf("%s/%s", ostreePrefix, encodeOStreeRef(dockerRef.String()))
	hasBranch, err := repo.hasBranch(branch)
	if err != nil {
		return err
	}
	if !hasBranch {
		return fmt.Errorf("cannot find the specified image")
	}

	_, imageID, err := repo.readMetadata(branch, "docker.digest")
	if err != nil {
		return err
	}

	imageID = strings.TrimPrefix(imageID, "sha256:")

	if imageID == ctr.Revision && len(set) == 0 {
		log.Println("latest version already deployed")
		return nil
	}

	mergedSet := make(map[string]string)
	for k, v := range ctr.Values {
		mergedSet[k] = fmt.Sprintf("%v", v)
	}
	for k, v := range set {
		mergedSet[k] = v
	}
	newDeployment, err := checkoutContainerTo(branch, repo, checkouts, mergedSet, name, image, imageID, nextRevision)
	if err != nil {
		return err
	}

	_, err = systemctlCommand("is-active", name, false, true)
	serviceActive := err == nil

	if err := destroyActiveCheckout(ctr, checkouts); err != nil {
		return err
	}
	if err := makeDeploymentActive(newDeployment, checkouts, name, false, nextRevision); err != nil {
		return err
	}
	if serviceActive {
		systemctlCommand("start", name, false, false)
	}
	return nil
}

func RollbackContainer(name string) error {
	checkouts := getCheckoutsDirectory()

	ctr, err := ReadContainer(checkouts, name, nil)
	if err != nil {
		return err
	}

	checkout := filepath.Join(checkouts, name)
	rev, err := getCurrentRevision(checkout)
	if err != nil {
		return err
	}
	nextRevision := ^rev & 0x1

	newDeployment, err := ReadContainer(checkouts, name, &nextRevision)
	if err != nil {
		return err
	}

	_, err = systemctlCommand("is-active", name, false, true)
	serviceActive := err == nil

	if err := destroyActiveCheckout(ctr, checkouts); err != nil {
		return err
	}
	if err := makeDeploymentActive(newDeployment, checkouts, name, false, nextRevision); err != nil {
		return err
	}
	if serviceActive {
		systemctlCommand("start", name, false, false)
	}
	return nil
}

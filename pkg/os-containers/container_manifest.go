package oscontainers

import (
	"encoding/json"
	"io/ioutil"
)

type ContainerManifest struct {
	Version                string            `json:"string"`
	DefaultValues          map[string]string `json:"defaultValues"`
	RenameFiles            map[string]string `json:"renameFiles"`
	NoContainerService     bool              `json:"noContainerService"`
	UseLinks               bool              `json:"useLinks"`
	InstalledFilesTemplate []string          `json:"installedFilesTemplate"`
}

func ReadContainerManifest(path string) (*ContainerManifest, error) {
	manifestBlob, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var schema ContainerManifest
	if json.Unmarshal(manifestBlob, &schema); err != nil {
		return nil, err
	}
	return &schema, nil
}

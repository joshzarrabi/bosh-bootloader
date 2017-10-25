package gcp

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/cloudfoundry/bosh-bootloader/storage"
)

var tempDir func(dir, prefix string) (string, error) = ioutil.TempDir
var writeFile func(file string, data []byte, perm os.FileMode) error = ioutil.WriteFile

type InputGenerator struct {
	gcpAvailabilityZoneRetriever gcpAvailabilityZoneRetriever
}

type gcpAvailabilityZoneRetriever interface {
	GetZones(string) ([]string, error)
}

func NewInputGenerator(availabilityZoneRetriever gcpAvailabilityZoneRetriever) InputGenerator {
	return InputGenerator{
		gcpAvailabilityZoneRetriever: availabilityZoneRetriever,
	}
}

func (i InputGenerator) Generate(state storage.State) (map[string]string, error) {
	azs, err := i.gcpAvailabilityZoneRetriever.GetZones(state.GCP.Region)
	if err != nil {
		return map[string]string{}, fmt.Errorf("Retrieving availability zones: %s", err)
	}
	if len(azs) == 0 {
		return map[string]string{}, errors.New("Zone list is empty")
	}
	zones, err := json.Marshal(azs)
	if err != nil {
		return map[string]string{}, err
	}

	dir, err := tempDir("", "")
	if err != nil {
		return map[string]string{}, err
	}

	credentialsPath := filepath.Join(dir, "credentials.json")
	err = writeFile(credentialsPath, []byte(state.GCP.ServiceAccountKey), os.ModePerm)
	if err != nil {
		return map[string]string{}, err
	}

	fmt.Println("------------------------------")
	fmt.Printf("zones: %s\n", string(zones))
	fmt.Printf("zone: %s\n", string(zones[0]))

	input := map[string]string{
		"env_id":             state.EnvID,
		"project_id":         state.GCP.ProjectID,
		"region":             state.GCP.Region,
		"zone":               azs[0],
		"availability_zones": string(zones),
		"credentials":        credentialsPath,
		"system_domain":      state.LB.Domain,
	}

	if state.LB.Cert != "" && state.LB.Key != "" {
		certPath := filepath.Join(dir, "cert")
		err = writeFile(certPath, []byte(state.LB.Cert), os.ModePerm)
		if err != nil {
			return map[string]string{}, err
		}
		input["ssl_certificate"] = certPath

		keyPath := filepath.Join(dir, "key")
		err = writeFile(keyPath, []byte(state.LB.Key), os.ModePerm)
		if err != nil {
			return map[string]string{}, err
		}
		input["ssl_certificate_private_key"] = keyPath
	}

	return input, nil
}

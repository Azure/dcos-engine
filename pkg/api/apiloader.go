package api

import (
	"encoding/json"
	"io/ioutil"
	"reflect"

	"github.com/Azure/dcos-engine/pkg/api/vlabs"
	"github.com/Azure/dcos-engine/pkg/helpers"
	"github.com/Azure/dcos-engine/pkg/i18n"
	log "github.com/sirupsen/logrus"
)

// Apiloader represents the object that loads api model
type Apiloader struct {
	Translator *i18n.Translator
}

// LoadContainerServiceFromFile loads an ACS Cluster API Model from a JSON file
func (a *Apiloader) LoadContainerServiceFromFile(jsonFile string, validate, isUpdate bool, existingContainerService *ContainerService) (*ContainerService, string, error) {
	contents, e := ioutil.ReadFile(jsonFile)
	if e != nil {
		return nil, "", a.Translator.Errorf("error reading file %s: %s", jsonFile, e.Error())
	}
	return a.DeserializeContainerService(contents, validate, isUpdate, existingContainerService)
}

// DeserializeContainerService loads an ACS Cluster API Model, validates it, and returns the unversioned representation
func (a *Apiloader) DeserializeContainerService(contents []byte, validate, isUpdate bool, existingContainerService *ContainerService) (*ContainerService, string, error) {
	m := &TypeMeta{}
	if err := json.Unmarshal(contents, &m); err != nil {
		return nil, "", err
	}

	version := m.APIVersion
	service, err := a.LoadContainerService(contents, version, validate, isUpdate, existingContainerService)
	if service == nil || err != nil {
		log.Infof("Error returned by LoadContainerService: %+v", err)
	}

	return service, version, err
}

// LoadContainerService loads an ACS Cluster API Model, validates it, and returns the unversioned representation
func (a *Apiloader) LoadContainerService(
	contents []byte,
	version string,
	validate, isUpdate bool,
	existingContainerService *ContainerService) (*ContainerService, error) {
	var curOrchVersion string
	hasExistingCS := existingContainerService != nil
	if hasExistingCS {
		curOrchVersion = existingContainerService.Properties.OrchestratorProfile.OrchestratorVersion
	}
	switch version {
	case vlabs.APIVersion:
		containerService := &vlabs.ContainerService{}
		if e := json.Unmarshal(contents, &containerService); e != nil {
			return nil, e
		}
		if e := checkJSONKeys(contents, reflect.TypeOf(*containerService), reflect.TypeOf(TypeMeta{})); e != nil {
			return nil, e
		}
		if hasExistingCS {
			vecs := ConvertContainerServiceToVLabs(existingContainerService)
			if e := containerService.Merge(vecs); e != nil {
				return nil, e
			}
		}
		if e := containerService.Properties.Validate(isUpdate); validate && e != nil {
			return nil, e
		}
		unversioned := ConvertVLabsContainerService(containerService)
		if curOrchVersion != "" &&
			(containerService.Properties.OrchestratorProfile == nil ||
				(containerService.Properties.OrchestratorProfile.OrchestratorVersion == "" &&
					containerService.Properties.OrchestratorProfile.OrchestratorRelease == "")) {
			unversioned.Properties.OrchestratorProfile.OrchestratorVersion = curOrchVersion
		}
		return unversioned, nil

	default:
		return nil, a.Translator.Errorf("unrecognized APIVersion '%s'", version)
	}
}

// SerializeContainerService takes an unversioned container service and returns the bytes
func (a *Apiloader) SerializeContainerService(containerService *ContainerService, version string) ([]byte, error) {
	switch version {
	case vlabs.APIVersion:
		vlabsContainerService := ConvertContainerServiceToVLabs(containerService)
		armContainerService := &VlabsARMContainerService{}
		armContainerService.ContainerService = vlabsContainerService
		armContainerService.APIVersion = version
		b, err := helpers.JSONMarshalIndent(armContainerService, "", "  ", false)
		if err != nil {
			return nil, err
		}
		return b, nil

	default:
		return nil, a.Translator.Errorf("invalid version %s for conversion back from unversioned object", version)
	}
}

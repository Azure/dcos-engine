package acsengine

import (
	"fmt"
	"path"

	"github.com/Azure/dcos-engine/pkg/api"
	"github.com/Azure/dcos-engine/pkg/i18n"
)

// ArtifactWriter represents the object that writes artifacts
type ArtifactWriter struct {
	Translator *i18n.Translator
}

// WriteTLSArtifacts saves TLS certificates and keys to the server filesystem
func (w *ArtifactWriter) WriteTLSArtifacts(containerService *api.ContainerService, apiVersion, template, parameters, artifactsDir string, certsGenerated bool, parametersOnly bool) error {
	if len(artifactsDir) == 0 {
		artifactsDir = fmt.Sprintf("%s-%s", containerService.Properties.OrchestratorProfile.OrchestratorType, GenerateClusterID(containerService.Properties))
		artifactsDir = path.Join("_output", artifactsDir)
	}

	f := &FileSaver{
		Translator: w.Translator,
	}

	// convert back the API object, and write it
	var b []byte
	var err error
	if !parametersOnly {
		apiloader := &api.Apiloader{
			Translator: w.Translator,
		}
		b, err = apiloader.SerializeContainerService(containerService, apiVersion)

		if err != nil {
			return err
		}

		if e := f.SaveFile(artifactsDir, "apimodel.json", b); e != nil {
			return e
		}

		if e := f.SaveFileString(artifactsDir, "azuredeploy.json", template); e != nil {
			return e
		}
	}

	if e := f.SaveFileString(artifactsDir, "azuredeploy.parameters.json", parameters); e != nil {
		return e
	}

	if !certsGenerated {
		return nil
	}
	return nil
}

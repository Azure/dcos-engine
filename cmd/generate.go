package cmd

import (
	"errors"
	"fmt"
	"os"
	"path"

	"github.com/Azure/dcos-engine/pkg/acsengine"
	"github.com/Azure/dcos-engine/pkg/acsengine/transform"
	"github.com/Azure/dcos-engine/pkg/api"
	"github.com/Azure/dcos-engine/pkg/i18n"
	"github.com/leonelquinteros/gotext"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const (
	generateName             = "generate"
	generateShortDescription = "Generate an Azure Resource Manager template"
	generateLongDescription  = "Generates an Azure Resource Manager template, parameters file and other assets for a cluster"
)

type generateCmd struct {
	apimodelPath    string
	outputDirectory string // can be auto-determined from clusterDefinition
	classicMode     bool
	noPrettyPrint   bool
	parametersOnly  bool
	set             []string

	// derived
	containerService *api.ContainerService
	apiVersion       string
	locale           *gotext.Locale
}

func newGenerateCmd() *cobra.Command {
	gc := generateCmd{}

	generateCmd := &cobra.Command{
		Use:   generateName,
		Short: generateShortDescription,
		Long:  generateLongDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := gc.validate(cmd, args); err != nil {
				log.Fatalf(fmt.Sprintf("error validating generateCmd: %s", err.Error()))
			}

			if err := gc.mergeAPIModel(); err != nil {
				log.Fatalf(fmt.Sprintf("error merging API model in generateCmd: %s", err.Error()))
			}

			if err := gc.loadAPIModel(cmd, args); err != nil {
				log.Fatalf(fmt.Sprintf("error loading API model in generateCmd: %s", err.Error()))
			}

			return gc.run()
		},
	}

	f := generateCmd.Flags()
	f.StringVar(&gc.apimodelPath, "api-model", "", "")
	f.StringVar(&gc.outputDirectory, "output-directory", "", "output directory (derived from FQDN if absent)")
	f.StringArrayVar(&gc.set, "set", []string{}, "set values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)")
	f.BoolVar(&gc.classicMode, "classic-mode", false, "enable classic parameters and outputs")
	f.BoolVar(&gc.noPrettyPrint, "no-pretty-print", false, "skip pretty printing the output")
	f.BoolVar(&gc.parametersOnly, "parameters-only", false, "only output parameters files")

	return generateCmd
}

func (gc *generateCmd) validate(cmd *cobra.Command, args []string) error {
	var err error

	gc.locale, err = i18n.LoadTranslations()
	if err != nil {
		return fmt.Errorf(fmt.Sprintf("error loading translation files: %s", err.Error()))
	}

	if gc.apimodelPath == "" {
		if len(args) == 1 {
			gc.apimodelPath = args[0]
		} else if len(args) > 1 {
			cmd.Usage()
			return errors.New("too many arguments were provided to 'generate'")
		} else {
			cmd.Usage()
			return errors.New("--api-model was not supplied, nor was one specified as a positional argument")
		}
	}

	if _, err := os.Stat(gc.apimodelPath); os.IsNotExist(err) {
		return fmt.Errorf(fmt.Sprintf("specified api model does not exist (%s)", gc.apimodelPath))
	}

	return nil
}

func (gc *generateCmd) mergeAPIModel() error {
	var err error

	// if --set flag has been used
	if gc.set != nil && len(gc.set) > 0 {
		m := make(map[string]transform.APIModelValue)
		transform.MapValues(m, gc.set)

		// overrides the api model and generates a new file
		gc.apimodelPath, err = transform.MergeValuesWithAPIModel(gc.apimodelPath, m)
		if err != nil {
			return fmt.Errorf(fmt.Sprintf("error merging --set values with the api model: %s", err.Error()))
		}

		log.Infoln(fmt.Sprintf("new api model file has been generated during merge: %s", gc.apimodelPath))
	}

	return nil
}

func (gc *generateCmd) loadAPIModel(cmd *cobra.Command, args []string) error {
	var err error

	apiloader := &api.Apiloader{
		Translator: &i18n.Translator{
			Locale: gc.locale,
		},
	}
	gc.containerService, gc.apiVersion, err = apiloader.LoadContainerServiceFromFile(gc.apimodelPath, true, false, nil)
	if err != nil {
		return fmt.Errorf(fmt.Sprintf("error parsing the api model: %s", err.Error()))
	}

	if gc.outputDirectory == "" {
		gc.outputDirectory = path.Join("_output", gc.containerService.Properties.MasterProfile.DNSPrefix)
	}
	return nil
}

func (gc *generateCmd) run() error {
	log.Infoln(fmt.Sprintf("Generating assets into %s...", gc.outputDirectory))

	ctx := acsengine.Context{
		Translator: &i18n.Translator{
			Locale: gc.locale,
		},
	}
	templateGenerator, err := acsengine.InitializeTemplateGenerator(ctx, gc.classicMode)
	if err != nil {
		log.Fatalf("failed to initialize template generator: %s", err.Error())
	}

	template, parameters, certsGenerated, err := templateGenerator.GenerateTemplate(gc.containerService, acsengine.DefaultGeneratorCode, false)
	if err != nil {
		log.Fatalf("error generating template %s: %s", gc.apimodelPath, err.Error())
		os.Exit(1)
	}

	if !gc.noPrettyPrint {
		if template, err = transform.PrettyPrintArmTemplate(template); err != nil {
			log.Fatalf("error pretty printing template: %s \n", err.Error())
		}
		if parameters, err = transform.BuildAzureParametersFile(parameters); err != nil {
			log.Fatalf("error pretty printing template parameters: %s \n", err.Error())
		}
	}

	writer := &acsengine.ArtifactWriter{
		Translator: &i18n.Translator{
			Locale: gc.locale,
		},
	}
	if err = writer.WriteTLSArtifacts(gc.containerService, gc.apiVersion, template, parameters, gc.outputDirectory, certsGenerated, gc.parametersOnly); err != nil {
		log.Fatalf("error writing artifacts: %s \n", err.Error())
	}

	return nil
}

package cmd

import (
	"fmt"
	"testing"

	"os"

	"github.com/Azure/dcos-engine/pkg/api"
	"github.com/Azure/dcos-engine/pkg/armhelpers"
	"github.com/satori/go.uuid"
	"github.com/spf13/cobra"
)

const ExampleAPIModel = `{
  "apiVersion": "vlabs",
  "properties": {
    "orchestratorProfile": {
      "orchestratorType": "DCOS",
	  "linuxBootstrapProfile" : { "bootstrapURL": "%s" }
    },
    "masterProfile": { "count": 1, "dnsPrefix": "", "vmSize": "Standard_D2_v2" },
    "agentPoolProfiles": [ { "name": "linuxpool1", "count": 2, "vmSize": "Standard_D2_v2", "availabilityProfile": "AvailabilitySet" } ],
    "windowsProfile": { "adminUsername": "azureuser", "adminPassword": "replacepassword1234$" },
    "linuxProfile": { "adminUsername": "azureuser", "ssh": { "publicKeys": [ { "keyData": "" } ] }
    }
  }
}
`

const ExampleAPIModelWithDNSPrefix = `{
  "apiVersion": "vlabs",
  "properties": {
    "orchestratorProfile": {
		"orchestratorType": "DCOS",
		"linuxBootstrapProfile" : { "bootstrapURL": "%s" }
	  },
    "masterProfile": { "count": 1, "dnsPrefix": "mytestcluster", "vmSize": "Standard_D2_v2" },
    "agentPoolProfiles": [ { "name": "linuxpool1", "count": 2, "vmSize": "Standard_D2_v2", "availabilityProfile": "AvailabilitySet" } ],
    "windowsProfile": { "adminUsername": "azureuser", "adminPassword": "replacepassword1234$" },
    "linuxProfile": { "adminUsername": "azureuser", "ssh": { "publicKeys": [ { "keyData": "" } ] }
	  }
	}
  }
  `

func getExampleAPIModel(dcosBootstrapURL string) string {
	return getAPIModel(ExampleAPIModel, dcosBootstrapURL)
}

func getAPIModel(baseAPIModel string, dcosBootstrapURL string) string {
	return fmt.Sprintf(baseAPIModel, dcosBootstrapURL)
}

func getAPIModelWithoutServicePrincipalProfile(baseAPIModel string, dcosBootstrapURL string) string {
	return fmt.Sprintf(baseAPIModel, dcosBootstrapURL)
}

func TestNewDeployCmd(t *testing.T) {
	output := newDeployCmd()
	if output.Use != deployName || output.Short != deployShortDescription || output.Long != deployLongDescription {
		t.Fatalf("deploy command should have use %s equal %s, short %s equal %s and long %s equal to %s", output.Use, deployName, output.Short, deployShortDescription, output.Long, versionLongDescription)
	}

	expectedFlags := []string{"api-model", "dns-prefix", "auto-suffix", "output-directory", "resource-group", "location", "force-overwrite"}
	for _, f := range expectedFlags {
		if output.Flags().Lookup(f) == nil {
			t.Fatalf("deploy command should have flag %s", f)
		}
	}
}

func TestValidate(t *testing.T) {
	r := &cobra.Command{}
	apimodelPath := "apimodel-unit-test.json"

	_, err := os.Create(apimodelPath)
	if err != nil {
		t.Fatalf("unable to create test apimodel path: %s", err.Error())
	}
	defer os.Remove(apimodelPath)

	cases := []struct {
		dc          *deployCmd
		expectedErr error
		args        []string
	}{
		{
			dc: &deployCmd{
				apimodelPath:    "",
				dnsPrefix:       "test",
				outputDirectory: "output/test",
				forceOverwrite:  false,
			},
			args:        []string{},
			expectedErr: fmt.Errorf("--api-model was not supplied, nor was one specified as a positional argument"),
		},
		{
			dc: &deployCmd{
				apimodelPath:    "",
				dnsPrefix:       "test",
				outputDirectory: "output/test",
			},
			args:        []string{"wrong/path"},
			expectedErr: fmt.Errorf("specified api model does not exist (wrong/path)"),
		},
		{
			dc: &deployCmd{
				apimodelPath:    "",
				dnsPrefix:       "test",
				outputDirectory: "output/test",
			},
			args:        []string{"test/apimodel.json", "some_random_stuff"},
			expectedErr: fmt.Errorf("too many arguments were provided to 'deploy'"),
		},
		{
			dc: &deployCmd{
				apimodelPath:    "",
				dnsPrefix:       "test",
				outputDirectory: "output/test",
			},
			args:        []string{apimodelPath},
			expectedErr: fmt.Errorf("--location must be specified"),
		},
		{
			dc: &deployCmd{
				apimodelPath:    "",
				dnsPrefix:       "test",
				outputDirectory: "output/test",
				location:        "west europe",
			},
			args:        []string{apimodelPath},
			expectedErr: nil,
		},
		{
			dc: &deployCmd{
				apimodelPath:    apimodelPath,
				dnsPrefix:       "test",
				outputDirectory: "output/test",
				location:        "canadaeast",
			},
			args:        []string{},
			expectedErr: nil,
		},
	}

	for _, c := range cases {
		err = c.dc.validateArgs(r, c.args)
		if err != nil && c.expectedErr != nil {
			if err.Error() != c.expectedErr.Error() {
				t.Fatalf("expected validate deploy command to return error %s, but instead got %s", c.expectedErr.Error(), err.Error())
			}
		} else {
			if c.expectedErr != nil {
				t.Fatalf("expected validate deploy command to return error %s, but instead got no error", c.expectedErr.Error())
			} else if err != nil {
				t.Fatalf("expected validate deploy command to return no error, but instead got %s", err.Error())
			}
		}
	}
}

func TestAutofillApimodelWithoutDcosBootstrapURL(t *testing.T) {
	testDeploy(t, "")
}

func TestAutofillApimodelWithDcosBootstrapURL(t *testing.T) {
	testDeploy(t, "abc.def")
}

func TestAutoSufixWithDnsPrefixInApiModel(t *testing.T) {
	apiloader := &api.Apiloader{
		Translator: nil,
	}

	apimodel := getAPIModel(ExampleAPIModelWithDNSPrefix, "")
	cs, ver, err := apiloader.DeserializeContainerService([]byte(apimodel), false, false, nil)
	if err != nil {
		t.Fatalf("unexpected error deserializing the example apimodel: %s", err)
	}
	deployCmd := &deployCmd{
		apimodelPath:     "./this/is/unused.json",
		outputDirectory:  "_test_output",
		forceOverwrite:   true,
		location:         "westus",
		autoSuffix:       true,
		containerService: cs,
		apiVersion:       ver,

		client: &armhelpers.MockACSEngineClient{},
	}

	err = autofillApimodel(deployCmd)
	if err != nil {
		t.Fatalf("unexpected error autofilling the example apimodel: %s", err)
	}

	defer os.RemoveAll(deployCmd.outputDirectory)

	if deployCmd.containerService.Properties.MasterProfile.DNSPrefix == "mytestcluster" {
		t.Fatalf("expected %s-{timestampsuffix} but got %s", "mytestcluster", deployCmd.containerService.Properties.MasterProfile.DNSPrefix)
	}

}

func testDeploy(t *testing.T, dcosBootstrapURL string) {
	apiloader := &api.Apiloader{
		Translator: nil,
	}

	apimodel := getExampleAPIModel(dcosBootstrapURL)
	cs, ver, err := apiloader.DeserializeContainerService([]byte(apimodel), false, false, nil)
	if err != nil {
		t.Fatalf("unexpected error deserializing the example apimodel: %s", err)
	}

	// deserialization happens in validate(), but we are testing just the default
	// setting that occurs in autofillApimodel (which is called from validate)
	// Thus, it assumes that containerService/apiVersion are already populated
	deployCmd := &deployCmd{
		apimodelPath:    "./this/is/unused.json",
		dnsPrefix:       "dnsPrefix1",
		outputDirectory: "_test_output",
		forceOverwrite:  true,
		location:        "westus",

		containerService: cs,
		apiVersion:       ver,

		client: &armhelpers.MockACSEngineClient{},
	}

	err = autofillApimodel(deployCmd)
	if err != nil {
		t.Fatalf("unexpected error autofilling the example apimodel: %s", err)
	}

	// cleanup, since auto-populations creates dirs and saves the SSH private key that it might create
	defer os.RemoveAll(deployCmd.outputDirectory)

	cs, _, err = deployCmd.validateApimodel()
	if err != nil {
		t.Fatalf("unexpected error validating apimodel after populating defaults: %s", err)
	}
}

func testDeployCmdMergeAPIModel(t *testing.T) {
	d := &deployCmd{}
	d.apimodelPath = "../pkg/acsengine/testdata/simple/dcos.json"
	err := d.mergeAPIModel()
	if err != nil {
		t.Fatalf("unexpected error calling mergeAPIModel with no --set flag defined: %s", err.Error())
	}

	d = &deployCmd{}
	d.apimodelPath = "../pkg/acsengine/testdata/simple/dcos.json"
	d.set = []string{"masterProfile.count=3,linuxProfile.adminUsername=testuser"}
	err = d.mergeAPIModel()
	if err != nil {
		t.Fatalf("unexpected error calling mergeAPIModel with one --set flag: %s", err.Error())
	}

	d = &deployCmd{}
	d.apimodelPath = "../pkg/acsengine/testdata/simple/dcos.json"
	d.set = []string{"masterProfile.count=3", "linuxProfile.adminUsername=testuser"}
	err = d.mergeAPIModel()
	if err != nil {
		t.Fatalf("unexpected error calling mergeAPIModel with multiple --set flags: %s", err.Error())
	}

	d = &deployCmd{}
	d.apimodelPath = "../pkg/acsengine/testdata/simple/dcos.json"
	d.set = []string{"agentPoolProfiles[0].count=1"}
	err = d.mergeAPIModel()
	if err != nil {
		t.Fatalf("unexpected error calling mergeAPIModel with one --set flag to override an array property: %s", err.Error())
	}
}

func testDeployCmdMLoadAPIModel(t *testing.T) {
	d := &deployCmd{}
	r := &cobra.Command{}
	f := r.Flags()

	addAuthFlags(&d.authArgs, f)

	fakeRawSubscriptionID := "6dc93fae-9a76-421f-bbe5-cc6460ea81cb"
	fakeSubscriptionID, err := uuid.FromString(fakeRawSubscriptionID)
	if err != nil {
		t.Fatalf("Invalid SubscriptionId in Test: %s", err)
	}

	d.apimodelPath = "../pkg/acsengine/testdata/simple/dcos.json"
	d.set = []string{"agentPoolProfiles[0].count=1"}
	d.SubscriptionID = fakeSubscriptionID
	d.rawSubscriptionID = fakeRawSubscriptionID

	d.validateArgs(r, []string{"../pkg/acsengine/testdata/simple/dcos.json"})
	d.mergeAPIModel()
	err = d.loadAPIModel(r, []string{"../pkg/acsengine/testdata/simple/dcos.json"})
	if err != nil {
		t.Fatalf("unexpected error loading api model: %s", err.Error())
	}
}

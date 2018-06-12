package armhelpers

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/Azure/azure-sdk-for-go/arm/authorization"
	"github.com/Azure/azure-sdk-for-go/arm/compute"
	"github.com/Azure/azure-sdk-for-go/arm/disk"
	"github.com/Azure/azure-sdk-for-go/arm/graphrbac"
	"github.com/Azure/azure-sdk-for-go/arm/resources/resources"
	"github.com/Azure/go-autorest/autorest"
)

//MockACSEngineClient is an implementation of ACSEngineClient where all requests error out
type MockACSEngineClient struct {
	FailDeployTemplate              bool
	FailDeployTemplateQuota         bool
	FailDeployTemplateConflict      bool
	FailEnsureResourceGroup         bool
	FailListVirtualMachines         bool
	FailListVirtualMachineScaleSets bool
	FailGetVirtualMachine           bool
	FailDeleteVirtualMachine        bool
	FailGetStorageClient            bool
	FailDeleteNetworkInterface      bool
	FailListProviders               bool
	ShouldSupportVMIdentity         bool
	FailDeleteRoleAssignment        bool
}

//MockStorageClient mock implementation of StorageClient
type MockStorageClient struct{}

//DeleteBlob mock
func (msc *MockStorageClient) DeleteBlob(container, blob string) error {
	return nil
}

//AddAcceptLanguages mock
func (mc *MockACSEngineClient) AddAcceptLanguages(languages []string) {}

//DeployTemplate mock
func (mc *MockACSEngineClient) DeployTemplate(resourceGroup, name string, template, parameters map[string]interface{}, cancel <-chan struct{}) (*resources.DeploymentExtended, error) {
	switch {
	case mc.FailDeployTemplate:
		return nil, errors.New("DeployTemplate failed")

	case mc.FailDeployTemplateQuota:
		errmsg := `resources.DeploymentsClient#CreateOrUpdate: Failure responding to request: StatusCode=400 -- Original Error: autorest/azure: Service returned an error.`
		resp := `{
"error":{
	"code":"InvalidTemplateDeployment",
	"message":"The template deployment is not valid according to the validation procedure. The tracking id is 'b5bd7d6b-fddf-4ec3-a3b0-ce285a48bd31'. See inner errors for details. Please see https://aka.ms/arm-deploy for usage details.",
	"details":[{
		"code":"QuotaExceeded",
		"message":"Operation results in exceeding quota limits of Core. Maximum allowed: 10, Current in use: 10, Additional requested: 2. Please read more about quota increase at http://aka.ms/corequotaincrease."
}]}}`

		return &resources.DeploymentExtended{
				Response: autorest.Response{
					Response: &http.Response{
						Status:     "400 Bad Request",
						StatusCode: 400,
						Body:       ioutil.NopCloser(bytes.NewReader([]byte(resp))),
					}}},
			errors.New(errmsg)

	case mc.FailDeployTemplateConflict:
		errmsg := `resources.DeploymentsClient#CreateOrUpdate: Failure sending request: StatusCode=200 -- Original Error: Long running operation terminated with status 'Failed': Code="DeploymentFailed" Message="At least one resource deployment operation failed. Please list deployment operations for details. Please see https://aka.ms/arm-debug for usage details.`
		resp := `{
"status":"Failed",
"error":{
	"code":"DeploymentFailed",
	"message":"At least one resource deployment operation failed. Please list deployment operations for details. Please see https://aka.ms/arm-debug for usage details.",
	"details":[{
		"code":"Conflict",
		"message":"{\r\n  \"error\": {\r\n    \"code\": \"PropertyChangeNotAllowed\",\r\n    \"target\": \"dataDisk.createOption\",\r\n    \"message\": \"Changing property 'dataDisk.createOption' is not allowed.\"\r\n  }\r\n}"
}]}}`
		return &resources.DeploymentExtended{
				Response: autorest.Response{
					Response: &http.Response{
						Status:     "200 OK",
						StatusCode: 200,
						Body:       ioutil.NopCloser(bytes.NewReader([]byte(resp))),
					}}},
			errors.New(errmsg)

	default:
		return nil, nil
	}
}

//EnsureResourceGroup mock
func (mc *MockACSEngineClient) EnsureResourceGroup(resourceGroup, location string, managedBy *string) (*resources.Group, error) {
	if mc.FailEnsureResourceGroup {
		return nil, fmt.Errorf("EnsureResourceGroup failed")
	}

	return nil, nil
}

//ListVirtualMachines mock
func (mc *MockACSEngineClient) ListVirtualMachines(resourceGroup string) (compute.VirtualMachineListResult, error) {
	if mc.FailListVirtualMachines {
		return compute.VirtualMachineListResult{}, fmt.Errorf("ListVirtualMachines failed")
	}

	vm1Name := "k8s-agentpool1-12345678-0"

	creationSourceString := "creationSource"
	orchestratorString := "orchestrator"
	resourceNameSuffixString := "resourceNameSuffix"
	poolnameString := "poolName"

	creationSource := "acsengine-k8s-agentpool1-12345678-0"
	orchestrator := "Kubernetes:1.6.9"
	resourceNameSuffix := "12345678"
	poolname := "agentpool1"

	tags := map[string]*string{
		creationSourceString:     &creationSource,
		orchestratorString:       &orchestrator,
		resourceNameSuffixString: &resourceNameSuffix,
		poolnameString:           &poolname,
	}

	vm1 := compute.VirtualMachine{
		Name: &vm1Name,
		Tags: &tags,
		VirtualMachineProperties: &compute.VirtualMachineProperties{
			StorageProfile: &compute.StorageProfile{
				OsDisk: &compute.OSDisk{
					Vhd: &compute.VirtualHardDisk{
						URI: &validOSDiskResourceName},
				},
			},
			NetworkProfile: &compute.NetworkProfile{
				NetworkInterfaces: &[]compute.NetworkInterfaceReference{
					{
						ID: &validNicResourceName,
					},
				},
			},
		},
	}

	vmr := compute.VirtualMachineListResult{}
	vmr.Value = &[]compute.VirtualMachine{vm1}

	return vmr, nil
}

//ListVirtualMachineScaleSets mock
func (mc *MockACSEngineClient) ListVirtualMachineScaleSets(resourceGroup string) (compute.VirtualMachineScaleSetListResult, error) {
	if mc.FailListVirtualMachineScaleSets {
		return compute.VirtualMachineScaleSetListResult{}, fmt.Errorf("ListVirtualMachines failed")
	}

	return compute.VirtualMachineScaleSetListResult{}, nil
}

//GetVirtualMachine mock
func (mc *MockACSEngineClient) GetVirtualMachine(resourceGroup, name string) (compute.VirtualMachine, error) {
	if mc.FailGetVirtualMachine {
		return compute.VirtualMachine{}, fmt.Errorf("GetVirtualMachine failed")
	}

	vm1Name := "k8s-agentpool1-12345678-0"

	creationSourceString := "creationSource"
	orchestratorString := "orchestrator"
	resourceNameSuffixString := "resourceNameSuffix"
	poolnameString := "poolName"

	creationSource := "acsengine-k8s-agentpool1-12345678-0"
	orchestrator := "Kubernetes:1.6.9"
	resourceNameSuffix := "12345678"
	poolname := "agentpool1"

	principalID := "00000000-1111-2222-3333-444444444444"

	tags := map[string]*string{
		creationSourceString:     &creationSource,
		orchestratorString:       &orchestrator,
		resourceNameSuffixString: &resourceNameSuffix,
		poolnameString:           &poolname,
	}

	var vmIdentity *compute.VirtualMachineIdentity
	if mc.ShouldSupportVMIdentity {
		vmIdentity = &compute.VirtualMachineIdentity{PrincipalID: &principalID}
	}

	return compute.VirtualMachine{
		Name:     &vm1Name,
		Tags:     &tags,
		Identity: vmIdentity,
		VirtualMachineProperties: &compute.VirtualMachineProperties{
			StorageProfile: &compute.StorageProfile{
				OsDisk: &compute.OSDisk{
					Vhd: &compute.VirtualHardDisk{
						URI: &validOSDiskResourceName},
				},
			},
			NetworkProfile: &compute.NetworkProfile{
				NetworkInterfaces: &[]compute.NetworkInterfaceReference{
					{
						ID: &validNicResourceName,
					},
				},
			},
		},
	}, nil
}

//DeleteVirtualMachine mock
func (mc *MockACSEngineClient) DeleteVirtualMachine(resourceGroup, name string, cancel <-chan struct{}) (<-chan compute.OperationStatusResponse, <-chan error) {
	if mc.FailDeleteVirtualMachine {
		errChan := make(chan error)
		respChan := make(chan compute.OperationStatusResponse)
		go func() {
			defer func() {
				close(errChan)
			}()
			defer func() {
				close(respChan)
			}()
			errChan <- fmt.Errorf("DeleteVirtualMachine failed")
		}()
		return respChan, errChan
	}

	errChan := make(chan error)
	respChan := make(chan compute.OperationStatusResponse)
	go func() {
		defer func() {
			close(errChan)
		}()
		defer func() {
			close(respChan)
		}()
		errChan <- nil
		respChan <- compute.OperationStatusResponse{}
	}()
	return respChan, errChan
}

//GetStorageClient mock
func (mc *MockACSEngineClient) GetStorageClient(resourceGroup, accountName string) (ACSStorageClient, error) {
	if mc.FailGetStorageClient {
		return nil, fmt.Errorf("GetStorageClient failed")
	}

	return &MockStorageClient{}, nil
}

//DeleteNetworkInterface mock
func (mc *MockACSEngineClient) DeleteNetworkInterface(resourceGroup, nicName string, cancel <-chan struct{}) (<-chan autorest.Response, <-chan error) {
	if mc.FailDeleteNetworkInterface {
		errChan := make(chan error)
		respChan := make(chan autorest.Response)
		go func() {
			defer func() {
				close(errChan)
			}()
			defer func() {
				close(respChan)
			}()
			errChan <- fmt.Errorf("DeleteNetworkInterface failed")
		}()
		return respChan, errChan
	}

	errChan := make(chan error)
	respChan := make(chan autorest.Response)
	go func() {
		defer func() {
			close(errChan)
		}()
		defer func() {
			close(respChan)
		}()
		errChan <- nil
		respChan <- autorest.Response{}
	}()
	return respChan, errChan
}

var validOSDiskResourceName = "https://00k71r4u927seqiagnt0.blob.core.windows.net/osdisk/k8s-agentpool1-12345678-0-osdisk.vhd"
var validNicResourceName = "/subscriptions/DEC923E3-1EF1-4745-9516-37906D56DEC4/resourceGroups/acsK8sTest/providers/Microsoft.Network/networkInterfaces/k8s-agent-12345678-nic-0"

// Active Directory
// Mocks

// Graph Mocks

// CreateGraphApplication creates an application via the graphrbac client
func (mc *MockACSEngineClient) CreateGraphApplication(applicationCreateParameters graphrbac.ApplicationCreateParameters) (graphrbac.Application, error) {
	return graphrbac.Application{}, nil
}

// CreateGraphPrincipal creates a service principal via the graphrbac client
func (mc *MockACSEngineClient) CreateGraphPrincipal(servicePrincipalCreateParameters graphrbac.ServicePrincipalCreateParameters) (graphrbac.ServicePrincipal, error) {
	return graphrbac.ServicePrincipal{}, nil
}

// CreateApp is a simpler method for creating an application
func (mc *MockACSEngineClient) CreateApp(applicationName, applicationURL string, replyURLs *[]string, requiredResourceAccess *[]graphrbac.RequiredResourceAccess) (applicationID, servicePrincipalObjectID, secret string, err error) {
	return "app-id", "client-id", "client-secret", nil
}

// RBAC Mocks

// CreateRoleAssignment creates a role assignment via the authorization client
func (mc *MockACSEngineClient) CreateRoleAssignment(scope string, roleAssignmentName string, parameters authorization.RoleAssignmentCreateParameters) (authorization.RoleAssignment, error) {
	return authorization.RoleAssignment{}, nil
}

// CreateRoleAssignmentSimple is a wrapper around RoleAssignmentsClient.Create
func (mc *MockACSEngineClient) CreateRoleAssignmentSimple(applicationID, roleID string) error {
	return nil
}

// DeleteManagedDisk is a wrapper around disksClient.Delete
func (mc *MockACSEngineClient) DeleteManagedDisk(resourceGroupName string, diskName string, cancel <-chan struct{}) (<-chan disk.OperationStatusResponse, <-chan error) {
	errChan := make(chan error)
	respChan := make(chan disk.OperationStatusResponse)
	go func() {
		defer func() {
			close(errChan)
		}()
		defer func() {
			close(respChan)
		}()
		errChan <- nil
		respChan <- disk.OperationStatusResponse{}
	}()
	return respChan, errChan
}

// ListManagedDisksByResourceGroup is a wrapper around disksClient.ListManagedDisksByResourceGroup
func (mc *MockACSEngineClient) ListManagedDisksByResourceGroup(resourceGroupName string) (result disk.ListType, err error) {
	return disk.ListType{}, nil
}

// ListProviders mock
func (mc *MockACSEngineClient) ListProviders() (resources.ProviderListResult, error) {
	if mc.FailListProviders {
		return resources.ProviderListResult{}, fmt.Errorf("ListProviders failed")
	}

	return resources.ProviderListResult{}, nil
}

// ListDeploymentOperations gets all deployments operations for a deployment.
func (mc *MockACSEngineClient) ListDeploymentOperations(resourceGroupName string, deploymentName string, top *int32) (result resources.DeploymentOperationsListResult, err error) {
	return resources.DeploymentOperationsListResult{}, nil
}

// ListDeploymentOperationsNextResults retrieves the next set of results, if any.
func (mc *MockACSEngineClient) ListDeploymentOperationsNextResults(lastResults resources.DeploymentOperationsListResult) (result resources.DeploymentOperationsListResult, err error) {
	return resources.DeploymentOperationsListResult{}, nil
}

// DeleteRoleAssignmentByID deletes a roleAssignment via its unique identifier
func (mc *MockACSEngineClient) DeleteRoleAssignmentByID(roleAssignmentID string) (authorization.RoleAssignment, error) {
	if mc.FailDeleteRoleAssignment {
		return authorization.RoleAssignment{}, fmt.Errorf("DeleteRoleAssignmentByID failed")
	}

	return authorization.RoleAssignment{}, nil
}

// ListRoleAssignmentsForPrincipal (e.g. a VM) via the scope and the unique identifier of the principal
func (mc *MockACSEngineClient) ListRoleAssignmentsForPrincipal(scope string, principalID string) (authorization.RoleAssignmentListResult, error) {
	roleAssignments := []authorization.RoleAssignment{}

	if mc.ShouldSupportVMIdentity {
		var assignmentID = "role-assignment-id"
		var assignment = authorization.RoleAssignment{
			ID: &assignmentID}
		roleAssignments = append(roleAssignments, assignment)
	}

	return authorization.RoleAssignmentListResult{
		Value: &roleAssignments}, nil
}

package api

import (
	"fmt"
	"strings"

	"github.com/Azure/dcos-engine/pkg/api/common"
	"github.com/Azure/dcos-engine/pkg/api/vlabs"
)

type orchestratorsFunc func(*OrchestratorProfile) ([]*OrchestratorVersionProfile, error)

var funcmap map[string]orchestratorsFunc
var versionsMap map[string][]string

func init() {
	funcmap = map[string]orchestratorsFunc{
		DCOS: dcosInfo,
	}
	versionsMap = map[string][]string{
		DCOS: common.GetAllSupportedDCOSVersions(),
	}
}

func validate(orchestrator, version string) (string, error) {
	switch {
	case strings.EqualFold(orchestrator, DCOS):
		return DCOS, nil
	case orchestrator == "":
		if version != "" {
			return "", fmt.Errorf("Must specify orchestrator for version '%s'", version)
		}
	default:
		return "", fmt.Errorf("Unsupported orchestrator '%s'", orchestrator)
	}
	return "", nil
}

func isVersionSupported(csOrch *OrchestratorProfile) bool {
	supported := false
	for _, version := range versionsMap[csOrch.OrchestratorType] {

		if version == csOrch.OrchestratorVersion {
			supported = true
			break
		}
	}
	return supported
}

// GetOrchestratorVersionProfileListVLabs returns vlabs OrchestratorVersionProfileList object per (optionally) specified orchestrator and version
func GetOrchestratorVersionProfileListVLabs(orchestrator, version string) (*vlabs.OrchestratorVersionProfileList, error) {
	apiOrchs, err := getOrchestratorVersionProfileList(orchestrator, version)
	if err != nil {
		return nil, err
	}
	orchList := &vlabs.OrchestratorVersionProfileList{}
	orchList.Orchestrators = []*vlabs.OrchestratorVersionProfile{}
	for _, orch := range apiOrchs {
		orchList.Orchestrators = append(orchList.Orchestrators, ConvertOrchestratorVersionProfileToVLabs(orch))
	}
	return orchList, nil
}

func getOrchestratorVersionProfileList(orchestrator, version string) ([]*OrchestratorVersionProfile, error) {
	var err error
	if orchestrator, err = validate(orchestrator, version); err != nil {
		return nil, err
	}
	orchs := []*OrchestratorVersionProfile{}
	if len(orchestrator) == 0 {
		// return all orchestrators
		for _, f := range funcmap {
			arr, err := f(&OrchestratorProfile{})
			if err != nil {
				return nil, err
			}
			orchs = append(orchs, arr...)
		}
	} else {
		if orchs, err = funcmap[orchestrator](&OrchestratorProfile{OrchestratorType: orchestrator, OrchestratorVersion: version}); err != nil {
			return nil, err
		}
	}
	return orchs, nil
}

// GetOrchestratorVersionProfile returns orchestrator info for upgradable container service
func GetOrchestratorVersionProfile(orch *OrchestratorProfile) (*OrchestratorVersionProfile, error) {
	if orch.OrchestratorVersion == "" {
		return nil, fmt.Errorf("Missing Orchestrator Version")
	}
	switch orch.OrchestratorType {
	case DCOS:
		arr, err := funcmap[orch.OrchestratorType](orch)
		if err != nil {
			return nil, err
		}
		// has to be exactly one element per specified orchestrator/version
		if len(arr) != 1 {
			return nil, fmt.Errorf("Umbiguous Orchestrator Versions")
		}
		return arr[0], nil
	default:
		return nil, fmt.Errorf("Upgrade operation is not supported for '%s'", orch.OrchestratorType)
	}
}

func dcosInfo(csOrch *OrchestratorProfile) ([]*OrchestratorVersionProfile, error) {
	orchs := []*OrchestratorVersionProfile{}
	if csOrch.OrchestratorVersion == "" {
		// get info for all supported versions
		for _, ver := range common.AllDCOSSupportedVersions {
			upgrades, err := dcosUpgrades(&OrchestratorProfile{OrchestratorVersion: ver})
			if err != nil {
				return nil, err
			}
			orchs = append(orchs,
				&OrchestratorVersionProfile{
					OrchestratorProfile: OrchestratorProfile{
						OrchestratorType:    DCOS,
						OrchestratorVersion: ver,
					},
					Default:  ver == common.DCOSDefaultVersion,
					Upgrades: upgrades,
				})
		}
	} else {
		if !isVersionSupported(csOrch) {
			return nil, fmt.Errorf("DCOS version %s is not supported", csOrch.OrchestratorVersion)
		}

		// get info for the specified version
		upgrades, err := dcosUpgrades(csOrch)
		if err != nil {
			return nil, err
		}
		orchs = append(orchs,
			&OrchestratorVersionProfile{
				OrchestratorProfile: OrchestratorProfile{
					OrchestratorType:    DCOS,
					OrchestratorVersion: csOrch.OrchestratorVersion,
				},
				Default:  csOrch.OrchestratorVersion == common.DCOSDefaultVersion,
				Upgrades: upgrades,
			})
	}
	return orchs, nil
}

func dcosUpgrades(csOrch *OrchestratorProfile) ([]*OrchestratorProfile, error) {
	ret := []*OrchestratorProfile{}

	switch csOrch.OrchestratorVersion {
	case common.DCOSVersion1Dot11Dot0:
		ret = append(ret, &OrchestratorProfile{
			OrchestratorType:    DCOS,
			OrchestratorVersion: common.DCOSVersion1Dot11Dot2,
		})
	}
	return ret, nil
}

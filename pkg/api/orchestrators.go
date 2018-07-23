package api

import (
	"fmt"
	"strconv"

	"github.com/Azure/dcos-engine/pkg/api/common"
	"github.com/Azure/dcos-engine/pkg/api/vlabs"
	"github.com/blang/semver"
)

func validate(version string) error {
	// for empty
	if len(version) == 0 {
		return nil
	}
	if !isVersionSupported(version) {
		return fmt.Errorf("DCOS version %s is not supported", version)
	}
	return nil
}

func isVersionSupported(version string) bool {
	for _, ver := range common.GetAllSupportedDCOSVersions() {

		if ver == version {
			return true
		}
	}
	return false
}

// GetOrchestratorVersionProfileListVLabs returns vlabs OrchestratorVersionProfileList object per (optionally) specified orchestrator and version
func GetOrchestratorVersionProfileListVLabs(version string) (*vlabs.OrchestratorVersionProfileList, error) {
	apiOrchs, err := getOrchestratorVersionProfileList(version)
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

func getOrchestratorVersionProfileList(version string) ([]*OrchestratorVersionProfile, error) {
	if err := validate(version); err != nil {
		return nil, err
	}
	orchs, err := dcosInfo(&OrchestratorProfile{OrchestratorType: DCOS, OrchestratorVersion: version})
	if err != nil {
		return nil, err
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
		arr, err := dcosInfo(orch)
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
		if !isVersionSupported(csOrch.OrchestratorVersion) {
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

	currentVer, err := semver.Make(csOrch.OrchestratorVersion)
	if err != nil {
		return nil, err
	}
	nextNextMinorString := strconv.FormatUint(currentVer.Major, 10) + "." + strconv.FormatUint(currentVer.Minor+2, 10) + ".0"
	upgradeableVersions := common.GetVersionsBetween(common.GetAllSupportedDCOSVersions(), csOrch.OrchestratorVersion, nextNextMinorString, false, false)
	for _, ver := range upgradeableVersions {
		ret = append(ret, &OrchestratorProfile{
			OrchestratorType:    DCOS,
			OrchestratorVersion: ver,
		})
	}
	return ret, nil
}

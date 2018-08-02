package cmd

import (
	"fmt"
	"sort"

	"github.com/Azure/dcos-engine/pkg/api"
	"github.com/Azure/dcos-engine/pkg/helpers"
	"github.com/blang/semver"
	"github.com/spf13/cobra"
)

const (
	orchestratorsName             = "orchestrator"
	orchestratorsShortDescription = "Display info about supported versions"
	orchestratorsLongDescription  = "Display supported versions and upgrade versions"
)

type orchestratorsCmd struct {
	// user input
	version string
}

type versionList struct {
	versions []semver.Version
}

func initVersionList(vers []string) (*versionList, error) {
	ret := &versionList{
		versions: []semver.Version{},
	}
	for _, ver := range vers {
		v, err := semver.Make(ver)
		if err != nil {
			return nil, err
		}
		ret.versions = append(ret.versions, v)
	}
	return ret, nil
}

func (h *versionList) Len() int {
	return len(h.versions)
}

func (h *versionList) Less(i, j int) bool {
	return h.versions[i].LT(h.versions[j])
}

func (h *versionList) Swap(i, j int) {
	h.versions[i], h.versions[j] = h.versions[j], h.versions[i]
}

func newOrchestratorsCmd() *cobra.Command {
	oc := orchestratorsCmd{}

	command := &cobra.Command{
		Use:   orchestratorsName,
		Short: orchestratorsShortDescription,
		Long:  orchestratorsLongDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			return oc.run(cmd, args)
		},
	}

	f := command.Flags()
	f.StringVar(&oc.version, "version", "", "orchestrator version (optional)")

	return command
}

func (oc *orchestratorsCmd) run(cmd *cobra.Command, args []string) error {
	orchs, err := api.GetOrchestratorVersionProfileListVLabs(oc.version)
	if err != nil {
		return err
	}

	for _, orch := range orchs.Orchestrators {
		list, err := initVersionList(orch.Upgrades)
		if err != nil {
			return err
		}
		sort.Sort(list)
		for i := range orch.Upgrades {
			orch.Upgrades[i] = list.versions[i].String()
		}
	}

	data, err := helpers.JSONMarshalIndent(orchs, "", "  ", false)
	if err != nil {
		return err
	}

	fmt.Println(string(data))
	return nil
}

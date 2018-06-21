package api

import (
	"testing"

	"github.com/Azure/dcos-engine/pkg/api/common"
	"github.com/blang/semver"
	. "github.com/onsi/gomega"
)

func TestInvalidVersion(t *testing.T) {
	RegisterTestingT(t)

	invalid := []string{
		"invalid number",
		"invalid.number",
		"a4.b7.c3",
		"31.29.",
		".17.02",
		"43.156.89.",
		"1.2.a"}

	for _, v := range invalid {
		_, e := semver.Make(v)
		Expect(e).NotTo(BeNil())
	}
}

func TestVersionCompare(t *testing.T) {
	RegisterTestingT(t)

	type record struct {
		v1, v2    string
		isGreater bool
	}
	records := []record{
		{"37.48.59", "37.48.59", false},
		{"17.4.5", "3.1.1", true},
		{"9.6.5", "9.45.5", false},
		{"2.3.8", "2.3.24", false}}

	for _, r := range records {
		ver, e := semver.Make(r.v1)
		Expect(e).To(BeNil())
		constraint, e := semver.Make(r.v2)
		Expect(e).To(BeNil())
		Expect(r.isGreater).To(Equal(ver.GT(constraint)))
	}
}

func TestDcosInfo(t *testing.T) {
	RegisterTestingT(t)
	invalid := []string{
		"invalid number",
		"invalid.number",
		"a4.b7.c3",
		"31.29.",
		".17.02",
		"43.156.89.",
		"1.2.a"}

	for _, v := range invalid {
		csOrch := &OrchestratorProfile{
			OrchestratorType:    DCOS,
			OrchestratorVersion: v,
		}

		_, e := dcosInfo(csOrch)
		Expect(e).NotTo(BeNil())
	}

	// test good value
	csOrch := &OrchestratorProfile{
		OrchestratorType:    DCOS,
		OrchestratorVersion: common.DCOSDefaultVersion,
	}

	_, e := dcosInfo(csOrch)
	Expect(e).To(BeNil())
}

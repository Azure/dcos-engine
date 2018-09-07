package cmd

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("The orchestrators command", func() {
	It("should create an orchestrators command", func() {
		output := newOrchestratorsCmd()

		Expect(output.Use).Should(Equal(orchestratorsName))
		Expect(output.Short).Should(Equal(orchestratorsShortDescription))
		Expect(output.Long).Should(Equal(orchestratorsLongDescription))
		Expect(output.Flags().Lookup("version")).NotTo(BeNil())
	})

	It("should fail on unsupported orchestrator", func() {
		command := &orchestratorsCmd{}

		err := command.run(nil, nil)
		Expect(err).To(BeNil())
	})

	It("should fail on unsupported version", func() {
		command := &orchestratorsCmd{
			version: "0.1.1",
		}

		err := command.run(nil, nil)
		Expect(err).NotTo(BeNil())
		Expect(err.Error()).To(Equal("DCOS version 0.1.1 is not supported"))
	})

	It("should succeed", func() {
		command := &orchestratorsCmd{
			version: "1.11.2",
		}

		err := command.run(nil, nil)
		Expect(err).To(BeNil())
	})
})

package domain_test

import (
	"flywheel/domain"
	"flywheel/domain/definition"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("WorkManager", func() {
	var (
		workManager *domain.WorkManager
	)
	BeforeEach(func() {
		workManager = &domain.WorkManager{}
	})

	Describe("Create", func() {
		It("should create new workManager successfully", func() {
			creation := &domain.WorkCreation{
				Name: "test work",
			}
			newWork := workManager.Create(creation)

			Ω(newWork).ShouldNot(BeZero())
			Ω(newWork.ID).Should(Equal(uint64(123)))
			Ω(newWork.Name).Should(Equal(creation.Name))
			Ω(newWork.CreateTime).ShouldNot(BeZero())
			Ω(newWork.Type).Should(Equal(&definition.GenericWorkType))
			Ω(newWork.State).Should(Equal(definition.GenericWorkType.StateMachine.States[0]))
			Ω(len(newWork.PropertyValues)).Should(Equal(0))
		})
	})
})

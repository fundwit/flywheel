package state_test

import (
	"flywheel/domain/state"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("StateMachine", func() {
	var (
		stateMachine *state.StateMachine
	)

	BeforeEach(func() {
		//         PENDING      DOING         DONE
		// PENDING   -            V (begin)   V (close)
		// DOING     V (cancel)   -           V (finish)
		// DONE      V (reopen)   X			  -
		stateMachine = state.NewStateMachine(
			[]state.State{{Name: "PENDING"}, {Name: "DOING"}, {Name: "DONE"}},
			[]state.Transition{
				{Name: "begin", From: "PENDING", To: "DOING"},
				{Name: "close", From: "PENDING", To: "DONE"},
				{Name: "cancel", From: "DOING", To: "PENDING"},
				{Name: "finish", From: "DOING", To: "DONE"},
				{Name: "reopen", From: "DONE", To: "PENDING"},
			})
	})

	Describe("NewStateMachine", func() {
		Context("With given PENDING-DOING-DONE states and transitions", func() {
			It("should create new State Machine successfully", func() {
				Expect(stateMachine).NotTo(BeZero())
				Expect(stateMachine.States).Should(Equal([]state.State{{Name: "PENDING"}, {Name: "DOING"}, {Name: "DONE"}}))
				Expect(stateMachine.Transitions).Should(Equal(
					[]state.Transition{
						{Name: "begin", From: "PENDING", To: "DOING"},
						{Name: "close", From: "PENDING", To: "DONE"},
						{Name: "cancel", From: "DOING", To: "PENDING"},
						{Name: "finish", From: "DOING", To: "DONE"},
						{Name: "reopen", From: "DONE", To: "PENDING"},
					},
				))
			})
		})
	})

	Describe("AvailableTransitions", func() {
		Context("With given PENDING-DOING-DONE states and transitions", func() {
			It("should return availableTransitions as expected", func() {
				Expect(stateMachine).NotTo(BeZero())

				Ω(stateMachine.AvailableTransitions("", "")).Should(Equal([]state.Transition{
					{Name: "begin", From: "PENDING", To: "DOING"},
					{Name: "close", From: "PENDING", To: "DONE"},
					{Name: "cancel", From: "DOING", To: "PENDING"},
					{Name: "finish", From: "DOING", To: "DONE"},
					{Name: "reopen", From: "DONE", To: "PENDING"},
				}))

				Ω(stateMachine.AvailableTransitions("PENDING", "")).Should(Equal([]state.Transition{
					{Name: "begin", From: "PENDING", To: "DOING"},
					{Name: "close", From: "PENDING", To: "DONE"},
				}))

				Ω(stateMachine.AvailableTransitions("", "PENDING")).Should(Equal([]state.Transition{
					{Name: "cancel", From: "DOING", To: "PENDING"},
					{Name: "reopen", From: "DONE", To: "PENDING"},
				}))

				Ω(stateMachine.AvailableTransitions("PENDING", "DOING")).Should(Equal([]state.Transition{
					{Name: "begin", From: "PENDING", To: "DOING"},
				}))

				Ω(stateMachine.AvailableTransitions("DOING", "")).Should(Equal([]state.Transition{
					{Name: "cancel", From: "DOING", To: "PENDING"},
					{Name: "finish", From: "DOING", To: "DONE"},
				}))

				Ω(stateMachine.AvailableTransitions("DONE", "")).Should(Equal([]state.Transition{
					{Name: "reopen", From: "DONE", To: "PENDING"},
				}))

				Ω(len(stateMachine.AvailableTransitions("UNKNOWN", ""))).Should(Equal(0))

			})
		})
	})
})

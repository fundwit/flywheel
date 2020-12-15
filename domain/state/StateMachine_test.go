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
				{Name: "begin", From: state.State{Name: "PENDING"}, To: state.State{Name: "DOING"}},
				{Name: "close", From: state.State{Name: "PENDING"}, To: state.State{Name: "DONE"}},
				{Name: "cancel", From: state.State{Name: "DOING"}, To: state.State{Name: "PENDING"}},
				{Name: "finish", From: state.State{Name: "DOING"}, To: state.State{Name: "DONE"}},
				{Name: "reopen", From: state.State{Name: "DONE"}, To: state.State{Name: "PENDING"}},
			})
	})

	Describe("NewStateMachine", func() {
		Context("With given PENDING-DOING-DONE states and transitions", func() {
			It("should create new State Machine successfully", func() {
				Expect(stateMachine).NotTo(BeZero())
				Expect(stateMachine.States).Should(Equal([]state.State{{Name: "PENDING"}, {Name: "DOING"}, {Name: "DONE"}}))
				Expect(stateMachine.Transitions).Should(Equal(
					[]state.Transition{
						{Name: "begin", From: state.State{Name: "PENDING"}, To: state.State{Name: "DOING"}},
						{Name: "close", From: state.State{Name: "PENDING"}, To: state.State{Name: "DONE"}},
						{Name: "cancel", From: state.State{Name: "DOING"}, To: state.State{Name: "PENDING"}},
						{Name: "finish", From: state.State{Name: "DOING"}, To: state.State{Name: "DONE"}},
						{Name: "reopen", From: state.State{Name: "DONE"}, To: state.State{Name: "PENDING"}},
					},
				))
			})
		})
	})

	Describe("AvailableTransitions", func() {
		Context("With given PENDING-DOING-DONE states and transitions", func() {
			It("should return availableTransitions as expected", func() {
				Expect(stateMachine).NotTo(BeZero())

				Ω(stateMachine.AvailableTransitions(state.State{Name: "PENDING"})).Should(Equal([]state.Transition{
					{Name: "begin", From: state.State{Name: "PENDING"}, To: state.State{Name: "DOING"}},
					{Name: "close", From: state.State{Name: "PENDING"}, To: state.State{Name: "DONE"}},
				}))

				Ω(stateMachine.AvailableTransitions(state.State{Name: "DOING"})).Should(Equal([]state.Transition{
					{Name: "cancel", From: state.State{Name: "DOING"}, To: state.State{Name: "PENDING"}},
					{Name: "finish", From: state.State{Name: "DOING"}, To: state.State{Name: "DONE"}},
				}))

				Ω(stateMachine.AvailableTransitions(state.State{Name: "DONE"})).Should(Equal([]state.Transition{
					{Name: "reopen", From: state.State{Name: "DONE"}, To: state.State{Name: "PENDING"}},
				}))

				Ω(len(stateMachine.AvailableTransitions(state.State{Name: "UNKNOWN"}))).Should(Equal(0))
			})
		})
	})
})

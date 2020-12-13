package state

// stateless object, just used for state computing
type StateMachine struct {
	States      []State
	Transitions []Transition
}

type State struct {
	Name string
}

type Transition struct {
	Name string
	From State
	To   State
}

func NewStateMachine(states []State, transitions []Transition) *StateMachine {
	return &StateMachine{States: states, Transitions: transitions}
}

func (sm *StateMachine) AvailableTransitions(state State) []Transition {
	var r []Transition
	for _, transition := range sm.Transitions {
		if transition.From == state {
			r = append(r, transition)
		}
	}
	return r
}

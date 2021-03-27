package state

import (
	"sort"
)

type StateMachineTraits interface {
	AvailableTransitions(state State) []Transition
}

// stateless object, just used for state computing
type StateMachine struct {
	States      []State      `json:"states"`
	Transitions []Transition `json:"transitions"`
}

type Category uint

const (
	InBacklog Category = 1
	InProcess Category = 2
	Done      Category = 3
)

type State struct {
	Name     string   `json:"name"`
	Category Category `json:"category"`
	Order    int      `json:"order"`
}

type Transition struct {
	Name string `json:"name" validate:"required"`
	From string `json:"from" validate:"required"`
	To   string `json:"to"   validate:"required"`
}

func NewStateMachine(states []State, transitions []Transition) *StateMachine {
	return &StateMachine{States: states, Transitions: transitions}
}

func (sm *StateMachine) FindState(stateName string) (State, bool) {
	for _, s := range sm.States {
		if stateName == s.Name {
			return s, true
		}
	}
	return State{}, false
}

func (sm *StateMachine) AvailableTransitions(fromState string, toState string) []Transition {
	r := []Transition{}
	for _, transition := range sm.Transitions {
		if (fromState == "" || fromState == transition.From) && (toState == "" || toState == transition.To) {
			r = append(r, transition)
		}
	}
	return r
}

type transitionList []Transition

func (I transitionList) Len() int {
	return len(I)
}
func (I transitionList) Less(i, j int) bool {
	if I[i].From == I[j].From {
		return I[i].To < I[j].To
	} else {
		return I[i].From < I[j].From
	}
}
func (I transitionList) Swap(i, j int) {
	I[i], I[j] = I[j], I[i]
}
func SortTransitions(transitions []Transition) []Transition {
	sorted := transitionList{}
	for _, t := range transitions {
		sorted = append(sorted, t)
	}
	sort.Sort(sorted)
	return sorted
}

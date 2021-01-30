package state

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
	InBacklog Category = iota
	InProcess
	Done
)

type State struct {
	Name     string   `json:"name"`
	Category Category `json:"category"`
}

type Transition struct {
	Name string `json:"name"`
	From State  `json:"from"`
	To   State  `json:"to"`
}

func NewStateMachine(states []State, transitions []Transition) *StateMachine {
	return &StateMachine{States: states, Transitions: transitions}
}

func (sm *StateMachine) AvailableTransitions(fromState string, toState string) []Transition {
	r := []Transition{}
	for _, transition := range sm.Transitions {
		if (fromState == "" || fromState == transition.From.Name) && (toState == "" || toState == transition.To.Name) {
			r = append(r, transition)
		}
	}
	return r
}

package linearreceipt

import (
	"fmt"
	"sort"

	"github.com/kastheco/kasmos/config/taskfsm"
	"github.com/kastheco/kasmos/config/taskstore"
)

// Config is the resolved [linear.receipts] block. The zero value is
// "feature disabled, no events, no state mappings".
type Config struct {
	Enabled  bool
	Events   map[taskfsm.Event]bool // empty when Enabled=true means default lifecycle set
	StateMap map[taskstore.Status]string
	// PRReceipts and MergeReceipts gate the side-effect notify helpers
	// independently of FSM lifecycle events.
	PRReceipts    bool
	MergeReceipts bool
	CancelReceipt bool
}

// TOMLBlock mirrors `[linear.receipts]` and `[linear.receipts.state_map]`.
type TOMLBlock struct {
	Enabled  bool              `toml:"enabled"`
	Events   []string          `toml:"events,omitempty"`
	StateMap map[string]string `toml:"state_map,omitempty"`
	PR       *bool             `toml:"pr,omitempty"`
	Merge    *bool             `toml:"merge,omitempty"`
	Cancel   *bool             `toml:"cancel,omitempty"`
}

func defaultEvents() []taskfsm.Event {
	return []taskfsm.Event{
		taskfsm.PlanStart,
		taskfsm.ImplementStart,
		taskfsm.ImplementFinished,
		taskfsm.ReviewApproved,
		taskfsm.ReviewChangesRequested,
		taskfsm.VerifyApproved,
		taskfsm.VerifyFailed,
		taskfsm.MarkDone,
		taskfsm.Cancel,
		taskfsm.Reopen,
	}
}

func validStatusKeys() []taskstore.Status {
	return []taskstore.Status{
		taskstore.StatusReady,
		taskstore.StatusPlanning,
		taskstore.StatusImplementing,
		taskstore.StatusReviewing,
		taskstore.StatusVerifying,
		taskstore.StatusDone,
		taskstore.StatusCancelled,
	}
}

// FromTOML parses and validates a TOMLBlock.
func FromTOML(b TOMLBlock) (Config, error) {
	if !b.Enabled {
		return Config{}, nil
	}

	cfg := Config{
		Enabled:       true,
		Events:        make(map[taskfsm.Event]bool),
		StateMap:      make(map[taskstore.Status]string),
		PRReceipts:    boolValue(b.PR, true),
		MergeReceipts: boolValue(b.Merge, true),
		CancelReceipt: boolValue(b.Cancel, true),
	}

	events := b.Events
	if len(events) == 0 {
		events = eventNames(defaultEvents())
	}
	for _, raw := range events {
		event, ok := taskfsm.EventByName(raw)
		if !ok {
			return Config{}, fmt.Errorf("linear receipts: unknown event %q", raw)
		}
		cfg.Events[event] = true
	}

	validStatuses := make(map[string]taskstore.Status, len(validStatusKeys()))
	for _, status := range validStatusKeys() {
		validStatuses[string(status)] = status
	}
	for rawStatus, stateID := range b.StateMap {
		status, ok := validStatuses[rawStatus]
		if !ok {
			return Config{}, fmt.Errorf("linear receipts: unknown state_map status %q", rawStatus)
		}
		if stateID == "" {
			return Config{}, fmt.Errorf("linear receipts: state_map status %q has empty Linear state id", rawStatus)
		}
		cfg.StateMap[status] = stateID
	}

	return cfg, nil
}

func boolValue(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func eventNames(events []taskfsm.Event) []string {
	names := make([]string, 0, len(events))
	for _, event := range events {
		names = append(names, string(event))
	}
	sort.Strings(names)
	return names
}

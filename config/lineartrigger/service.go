package lineartrigger

import (
	"context"
	"errors"

	"github.com/kastheco/kasmos/config/linearlink"
	"github.com/kastheco/kasmos/config/taskfsm"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/internal/linear"
)

// Outcome is the pure dispatch decision returned for a parsed Linear trigger.
type Outcome struct {
	Kind          OutcomeKind
	HelpReply     string
	StatusReply   string
	CreateInput   *linearlink.CreateFromIssueInput
	LinkInput     *linearlink.LinkInput
	StartSignal   *GatewayEmit
	Reject        *RejectDetail
	IgnoredReason string
}

// OutcomeKind names the action a Poller should execute for a trigger.
type OutcomeKind string

const (
	OutcomeHelp         OutcomeKind = "help"
	OutcomeStatusReply  OutcomeKind = "status"
	OutcomeCreate       OutcomeKind = "create"
	OutcomeLink         OutcomeKind = "link"
	OutcomePlanRequest  OutcomeKind = "plan"
	OutcomeStartRequest OutcomeKind = "start"
	OutcomeRejected     OutcomeKind = "rejected"
	OutcomeIgnored      OutcomeKind = "ignored"
)

// GatewayEmit carries the task-store signal gateway insert requested by a trigger.
type GatewayEmit struct {
	Project    string
	PlanFile   string
	SignalType string
	Payload    string
}

// RejectDetail carries a stable rejection reason and formatted reply.
type RejectDetail struct {
	Reason string
	Body   string
}

// Service is the pure Linear trigger dispatcher. It reads task-store state but
// does not mutate it, call Linear, or emit task FSM signals.
type Service struct {
	cfg        Config
	project    string
	router     *Router
	authoriser *Authoriser
	validator  *Validator
	store      taskstore.Store
}

// NewService returns a pure dispatcher with default helpers filled from cfg.
func NewService(project string, cfg Config, store taskstore.Store, router *Router, authoriser *Authoriser, validator *Validator) *Service {
	if router == nil {
		router = NewRouter(cfg)
	}
	if authoriser == nil {
		authoriser = NewAuthoriser(cfg)
	}
	if validator == nil {
		validator = NewValidator(cfg, store, project)
	}
	return &Service{
		cfg:        cfg,
		project:    project,
		router:     router,
		authoriser: authoriser,
		validator:  validator,
		store:      store,
	}
}

// Decide produces an Outcome for a parsed intent and polled issue.
func (s *Service) Decide(ctx context.Context, intent ParsedIntent, issue linear.Issue) Outcome {
	if intent.Verb == VerbHelp {
		return Outcome{Kind: OutcomeHelp, HelpReply: FormatHelp(HelpInput{EnabledVerbs: enabledVerbs(s.cfg.Verbs), Labels: s.cfg.Labels})}
	}
	if !s.cfg.Verbs[intent.Verb] {
		if intent.Source == SourceLabel {
			return Outcome{Kind: OutcomeIgnored, IgnoredReason: "verb_disabled"}
		}
		return s.reject(intent.Verb, "verb_disabled")
	}

	if ok, reason := s.authoriser.Allow(intent, intent.Verb); !ok {
		if reason == "" {
			reason = "actor_not_allowed"
		}
		return s.reject(intent.Verb, reason)
	}

	route := s.router.Resolve(issue)
	if route.Match == nil {
		reason := route.Reason
		if reason == "" {
			reason = "route_missing"
		}
		return s.reject(intent.Verb, reason)
	}

	switch intent.Verb {
	case VerbStatus:
		entry, ok := s.linkedEntry(issue.ID)
		if !ok {
			return s.reject(intent.Verb, "unlinked_target")
		}
		return Outcome{Kind: OutcomeStatusReply, StatusReply: FormatStatus(statusInput(entry, issue))}
	case VerbLink:
		if intent.TaskFileArg == "" {
			return s.reject(intent.Verb, "missing_task_file")
		}
		entry, err := s.store.Get(s.project, intent.TaskFileArg)
		if err != nil {
			return s.reject(intent.Verb, "unlinked_target")
		}
		if result := s.validator.Validate(VerbLink, entry, issue); !result.OK {
			return s.reject(intent.Verb, result.Reason)
		}
		return Outcome{Kind: OutcomeLink, LinkInput: &linearlink.LinkInput{
			Filename: entry.Filename,
			IssueArg: issue.ID,
			Reason:   "linear-trigger-link",
		}}
	case VerbCreate:
		if _, ok := s.linkedEntry(issue.ID); ok {
			return s.reject(intent.Verb, "duplicate_link")
		}
		return Outcome{Kind: OutcomeCreate, CreateInput: &linearlink.CreateFromIssueInput{
			IssueArg:     issue.ID,
			Filename:     intent.TaskFileArg,
			Topic:        route.Match.Topic,
			BranchPrefix: route.Match.BranchPrefix,
			Reason:       "linear-trigger-create",
		}}
	case VerbPlan:
		entry, ok := s.linkedEntry(issue.ID)
		if !ok {
			return Outcome{
				Kind: OutcomePlanRequest,
				CreateInput: &linearlink.CreateFromIssueInput{
					IssueArg:     issue.ID,
					Filename:     intent.TaskFileArg,
					Topic:        route.Match.Topic,
					BranchPrefix: route.Match.BranchPrefix,
					Reason:       "linear-trigger-plan-create",
				},
				StartSignal: &GatewayEmit{
					Project:    s.project,
					SignalType: string(taskfsm.PlanStart),
				},
			}
		}
		if result := s.validator.Validate(VerbPlan, entry, issue); !result.OK {
			return s.reject(intent.Verb, result.Reason)
		}
		return Outcome{Kind: OutcomePlanRequest, StartSignal: &GatewayEmit{
			Project:    s.project,
			PlanFile:   entry.Filename,
			SignalType: string(taskfsm.PlanStart),
		}}
	case VerbStart:
		entry, ok := s.linkedEntry(issue.ID)
		if !ok {
			return s.reject(intent.Verb, "unlinked_target")
		}
		if result := s.validator.Validate(VerbStart, entry, issue); !result.OK {
			return s.reject(intent.Verb, result.Reason)
		}
		return Outcome{Kind: OutcomeStartRequest, StartSignal: &GatewayEmit{
			Project:    s.project,
			PlanFile:   entry.Filename,
			SignalType: string(taskfsm.ImplementStart),
		}}
	default:
		return s.reject(intent.Verb, "invalid_command")
	}
}

func (s *Service) linkedEntry(issueID string) (taskstore.TaskEntry, bool) {
	filename, err := s.store.FindLinkedTask(s.project, issueID,
		taskstore.StatusReady,
		taskstore.StatusPlanning,
		taskstore.StatusImplementing,
		taskstore.StatusReviewing,
		taskstore.StatusVerifying,
	)
	if errors.Is(err, taskstore.ErrNotFound) || filename == "" {
		return taskstore.TaskEntry{}, false
	}
	if err != nil {
		return taskstore.TaskEntry{}, false
	}
	entry, err := s.store.Get(s.project, filename)
	if err != nil {
		return taskstore.TaskEntry{}, false
	}
	return entry, true
}

func (s *Service) reject(verb Verb, reason string) Outcome {
	if reason == "" {
		reason = "invalid_command"
	}
	return Outcome{Kind: OutcomeRejected, Reject: &RejectDetail{
		Reason: reason,
		Body:   FormatReject(RejectInput{Verb: verb, Reason: reason}),
	}}
}

func enabledVerbs(verbs map[Verb]bool) []Verb {
	out := make([]Verb, 0, len(verbs))
	for _, verb := range AllVerbs() {
		if verbs[verb] {
			out = append(out, verb)
		}
	}
	return out
}

func statusInput(entry taskstore.TaskEntry, issue linear.Issue) StatusInput {
	return StatusInput{
		Filename:       entry.Filename,
		Branch:         entry.Branch,
		Status:         entry.Status,
		ExecutionPhase: entry.ExecutionState.Phase,
		ActiveAgent:    entry.ExecutionState.ActiveAgentType,
		ActiveWave:     entry.ExecutionState.ActiveWave,
		PRURL:          entry.PRURL,
		ReviewBody:     entry.LatestReviewFeedback,
		Identifier:     issue.Identifier,
	}
}

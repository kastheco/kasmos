package linearreceipt

import (
	"github.com/kastheco/kasmos/config/auditlog"
	"github.com/kastheco/kasmos/config/taskfsm"
	"github.com/kastheco/kasmos/config/taskstore"
)

// BuildRegistryWithReceipts appends the Linear receipt hook to baseHooks when
// receipts are enabled. A nil/disabled client keeps the original registry.
func BuildRegistryWithReceipts(baseHooks *taskfsm.HookRegistry, cfg Config, store taskstore.Store, client ClientAdapter, logger auditlog.Logger, project string) *taskfsm.HookRegistry {
	if !cfg.Enabled || store == nil || client == nil {
		return baseHooks
	}
	if baseHooks == nil {
		baseHooks = taskfsm.NewHookRegistry()
	}
	baseHooks.Add(NewHook(cfg, store, client, logger, project), nil)
	return baseHooks
}

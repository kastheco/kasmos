package appwidget

import "github.com/kastheco/kasmos/internal/livestatus"

// MonitorContractVersion is the KasmosMonitorHost API version.
const MonitorContractVersion = 1

// SnapshotPath is the stable read-only HTTP bridge for host-mounted monitor panes.
const SnapshotPath = "/v1/monitor/snapshot"

// DefaultSnapshotEndpoint is the loopback default a host uses when none is configured.
const DefaultSnapshotEndpoint = "http://127.0.0.1:7433" + SnapshotPath

// LiveStatusSchemaVersion re-exports the wire schema version the bundle can parse.
const LiveStatusSchemaVersion = livestatus.SchemaVersion

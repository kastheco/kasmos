package webassets

import _ "embed"

//go:embed admin/widget-dist/monitor.js
var monitorJS string

//go:embed admin/widget-dist/monitor.css
var monitorCSS string

// MonitorBundle returns the committed monitor widget bundle (js, css).
func MonitorBundle() (string, string) { return monitorJS, monitorCSS }

package appwidget

import (
	"encoding/json"
	"os"
	"strings"

	webassets "github.com/kastheco/kasmos/web"
)

const WidgetURI = "ui://widget/kasmos-monitor.html"
const DefaultPreviewEndpoint = "http://127.0.0.1:7433" + PreviewPath

// ResourceMIMEType returns the Apps SDK MIME type, overridable for host compatibility.
func ResourceMIMEType() string {
	if value := strings.TrimSpace(os.Getenv("KASMOS_APP_WIDGET_MIME")); value != "" {
		return value
	}
	return "text/html;profile=mcp-app"
}

// WidgetHTML returns the fully inlined widget document.
func WidgetHTML() string {
	js, css := webassets.MonitorBundle()
	return `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"></head><body><div id="root"></div><style>` + css + `</style><script type="module">` + js + `</script></body></html>`
}

const previewShim = `<script>
const kasmosPreviewEndpoint=__KASMOS_PREVIEW_ENDPOINT__;
const kasmosPreviewInput=__KASMOS_PREVIEW_INPUT__;
window.openai={displayMode:"inline",theme:"light",toolInput:kasmosPreviewInput,widgetState:kasmosPreviewInput,callTool:async function(name,args){if(name!=="refresh_monitor")throw new Error("preview only supports refresh_monitor");const response=await fetch(kasmosPreviewEndpoint,{method:"POST",headers:{"content-type":"application/json"},body:JSON.stringify(args||{})});const payload=await response.json();if(!response.ok)throw new Error("refresh_monitor failed: "+response.status);return payload},setWidgetState:function(state){this.widgetState=state},requestDisplayMode:async function(request){const mode=request.mode;this.displayMode=mode;document.documentElement.classList.toggle("pip",mode==="pip");window.dispatchEvent(new CustomEvent("openai:set_globals",{detail:{globals:{displayMode:mode}}}));return {mode:mode}},sendFollowUpMessage:function(message){console.log(message)}};
</script>`

// PreviewHTML returns the widget with a browser-host shim injected before its module.
func PreviewHTML() string {
	return PreviewHTMLWithInput("", "")
}

// PreviewHTMLWithInput returns the widget with seeded tool input for standalone previews.
func PreviewHTMLWithInput(project, task string) string {
	return PreviewHTMLWithEndpoint(project, task, DefaultPreviewEndpoint)
}

// PreviewHTMLWithEndpoint returns the widget with seeded input and a custom preview bridge endpoint.
func PreviewHTMLWithEndpoint(project, task, endpoint string) string {
	input, _ := json.Marshal(struct {
		Project string `json:"project,omitempty"`
		Task    string `json:"task,omitempty"`
	}{Project: project, Task: task})
	encodedEndpoint, _ := json.Marshal(endpoint)
	shim := strings.Replace(previewShim, "__KASMOS_PREVIEW_INPUT__", string(input), 1)
	shim = strings.Replace(shim, "__KASMOS_PREVIEW_ENDPOINT__", string(encodedEndpoint), 1)
	return strings.Replace(WidgetHTML(), `<script type="module">`, shim+`<script type="module">`, 1)
}

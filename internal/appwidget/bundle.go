package appwidget

import (
	"os"
	"strings"

	webassets "github.com/kastheco/kasmos/web"
)

const WidgetURI = "ui://widget/kasmos-monitor.html"

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
const kasmosPreviewEndpoint="http://127.0.0.1:7433/v1/widget-preview/open-monitor";
window.openai={displayMode:"inline",theme:"light",widgetState:{},callTool:async function(name,args){if(name!=="open_monitor")throw new Error("preview only supports open_monitor");const response=await fetch(kasmosPreviewEndpoint,{method:"POST",headers:{"content-type":"application/json"},body:JSON.stringify(args||{})});const payload=await response.json();if(!response.ok)throw new Error("open_monitor failed: "+response.status);return payload},setWidgetState:function(state){this.widgetState=state},requestDisplayMode:async function(mode){this.displayMode=mode;document.documentElement.classList.toggle("pip",mode==="pip");return {mode:mode}},sendFollowUpMessage:function(message){console.log(message)}};
</script>`

// PreviewHTML returns the widget with a browser-host shim injected before its module.
func PreviewHTML() string {
	return strings.Replace(WidgetHTML(), `<script type="module">`, previewShim+`<script type="module">`, 1)
}

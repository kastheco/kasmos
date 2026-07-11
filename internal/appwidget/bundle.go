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
const kasmosMCP={endpoint:"http://127.0.0.1:7434/mcp",session:"",nextId:1,headers:function(){const headers={"content-type":"application/json","accept":"application/json, text/event-stream"};if(this.session)headers["mcp-session-id"]=this.session;return headers},parse:async function(response){const text=await response.text();const line=text.split("\n").find(function(value){return value.startsWith("data: ")});return JSON.parse(line?line.slice(6):text)},post:async function(method,params){const response=await fetch(this.endpoint,{method:"POST",headers:this.headers(),body:JSON.stringify({jsonrpc:"2.0",id:this.nextId++,method:method,params:params||{}})});if(!response.ok)throw new Error("mcp request failed: "+response.status);const session=response.headers.get("mcp-session-id");if(session)this.session=session;return this.parse(response)},notify:async function(method){const response=await fetch(this.endpoint,{method:"POST",headers:this.headers(),body:JSON.stringify({jsonrpc:"2.0",method:method})});if(!response.ok)throw new Error("mcp notification failed: "+response.status)},initialize:async function(){if(this.session)return;await this.post("initialize",{protocolVersion:"2025-03-26",capabilities:{},clientInfo:{name:"kasmos-widget-preview",version:"1"}});await this.notify("notifications/initialized")}};
window.openai={displayMode:"inline",theme:"light",widgetState:{},callTool:async function(name,args){await kasmosMCP.initialize();const payload=await kasmosMCP.post("tools/call",{name:name,arguments:args||{}});return payload.result},setWidgetState:function(state){this.widgetState=state},requestDisplayMode:async function(mode){this.displayMode=mode;document.documentElement.classList.toggle("pip",mode==="pip");return {mode:mode}},sendFollowUpMessage:function(message){console.log(message)}};
</script>`

// PreviewHTML returns the widget with a browser-host shim injected before its module.
func PreviewHTML() string {
	return strings.Replace(WidgetHTML(), `<script type="module">`, previewShim+`<script type="module">`, 1)
}

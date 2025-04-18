# Websocket - Full Usage Example

```go
package main

import (
 "log"

 "5tk.dev/braza"
)


func main() {
 app := braza.NewApp(nil)
 app.AddRoute(braza.Get("/", home))
 app.AddRoute(&braza.Route{
  Url:  "/echo",
  Name: "echo",
  Func: echo,
 })
 app.Listen()
}

func echo(ctx *braza.Ctx) {
 c, err := ctx.Request.Websocket(nil)
 if err != nil {
  log.Print("upgrade:", err)
  return
 }
 defer c.Close()
 for {
  mt, message, err := c.ReadMessage()
  if err != nil {
   log.Println("read:", err)
   break
  }
  log.Printf("recv: %s", message)
  err = c.WriteMessage(mt, message)
  if err != nil {
   log.Println("write:", err)
   break
  }
 }
}

func home(ctx *braza.Ctx) {
 ctx.Response.RenderTemplate("home.html")
}
```

## *templates/home.html*

```html
<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<script>
window.addEventListener("load", function(evt) {

    var output = document.getElementById("output");
    var input = document.getElementById("input");
    var ws;

    var print = function(message) {
        var d = document.createElement("div");
        d.textContent = message;
        output.appendChild(d);
        output.scroll(0, output.scrollHeight);
    };

    document.getElementById("open").onclick = function(evt) {
        if (ws) {
            return false;
        }
        ws = new WebSocket("ws://localhost:5000/echo");
        ws.onopen = function(evt) {
            print("OPEN");
        }
        ws.onclose = function(evt) {
            print("CLOSE");
            ws = null;
        }
        ws.onmessage = function(evt) {
            print("RESPONSE: " + evt.data);
        }
        ws.onerror = function(evt) {
            print("ERROR: " + evt.data);
        }
        return false;
    };

    document.getElementById("send").onclick = function(evt) {
        if (!ws) {
            return false;
        }
        print("SEND: " + input.value);
        ws.send(input.value);
        return false;
    };

    document.getElementById("close").onclick = function(evt) {
        if (!ws) {
            return false;
        }
        ws.close();
        return false;
    };

});
</script>
</head>
<body>
<table>
<tr><td valign="top" width="50%">
<p>Click "Open" to create a connection to the server,
"Send" to send a message to the server and "Close" to close the connection.
You can change the message and send multiple times.
<p>
<form>
<button id="open">Open</button>
<button id="close">Close</button>
<p><input id="input" type="text" value="Hello world!">
<button id="send">Send</button>
</form>
</td><td valign="top" width="50%">
<div id="output" style="max-height: 70vh;overflow-y: scroll;"></div>
</td></tr></table>
</body>
</html>
`
```

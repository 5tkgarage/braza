# Middleware - Full Usage Example

```go
package main

import (
 "fmt"

 "5tk.dev/braza"
)

func main() {
 app := braza.NewApp(nil)
 app.Middlewares = braza.Middlewares{middle1, middle2}

 app.AddRoute(&braza.Route{
  Url:         "/",
  Func:        home,
  Middlewares: braza.Middlewares{middle3, middle4},
 })

 app.AddRoute(&braza.Route{
  Url:  "/echo",
  Name: "echo",
  Func: echo,
 })
 app.Listen()
}

func home(ctx *braza.Ctx) {
 ctx.response.HTML("<h1>Hello</h1>",200)
}


func middle1(ctx *braza.Ctx) {
 fmt.Println("middle 1")
 ctx.Next()
}

func middle2(ctx *braza.Ctx) {
 fmt.Println("middle 2")
 ctx.Next()
}

func middle3(ctx *braza.Ctx) {
 fmt.Println("middle 3")
 ctx.Next()
}
func middle4(ctx *braza.Ctx) {
 fmt.Println("middle 4")
 ctx.Next()
}
```

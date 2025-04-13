package main

import (
	"fmt"

	"5tk.dev/braza"
)

var db = map[string]string{}

func main() {
	app := braza.NewApp(nil)

	app.Prefix = "/v1"           // url prefix for this router
	app.Subdomain = "api"        // subdomain for this router
	app.Servername = "localhost" // servername for match subdomains. http port is not required

	app.Cors = &braza.Cors{
		AllowOrigins: []string{"*"},
		AllowHeaders: []string{"Authorization", "*"},
	}

	app.GET("/todo", get)
	app.POST("/todo", post)

	app.PUT("/todo/{id:int}", put)
	app.DELETE("/todo/{id:int}", del)

	app.Listen()
}

func get(ctx *braza.Ctx) {
	ctx.JSON(db, 200)
}

func post(ctx *braza.Ctx) {
	item, ok := ctx.Request.Form["item"].(string)
	if !ok && item == "" {
		ctx.BadRequest()
	}
	db[fmt.Sprint(len(db))] = item
	ctx.JSON(db, 200)
}

func put(ctx *braza.Ctx) {
	id := ctx.Request.PathArgs["id"]
	item, ok := ctx.Request.Form["item"].(string)
	if !ok && item == "" {
		ctx.BadRequest()
	}

	if _, ok := db[id]; !ok {
		ctx.NotFound()
	}
	db[id] = item
	ctx.JSON(db, 200)
}

func del(ctx *braza.Ctx) {
	id := ctx.Request.PathArgs["id"]
	if _, ok := db[id]; ok {
		delete(db, id)
		ctx.NoContent()
	}
	ctx.NotFound()
}

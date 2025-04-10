package braza

import (
	"context"
	"net/http"
	"slices"

	"github.com/5tkgarage/c3po"
	"github.com/golang-jwt/jwt/v5"
)

type abortCode int

// Returns a new *braza.Ctx
func NewCtx(app *App, wr http.ResponseWriter, rq *http.Request) *Ctx {
	ctx := &Ctx{
		App:    app,
		Global: map[string]any{},
		Session: &Session{
			del:    []string{},
			claims: jwt.MapClaims{},
		},
		MatchInfo: &MatchInfo{},
	}

	ctx.Request = NewRequest(rq, ctx)
	ctx.Response = NewResponse(wr, ctx)

	c := context.Background()
	ctx.backCtx = context.WithValue(c, abortCode(1), nil)

	return ctx
}

type Ctx struct {
	// Clone Current App
	App *App

	/*
		global variables of current request
			app.BeforeRequest = (ctx *braza.Ctx){
				db := database.Open().Session()
				ctx.Global["db"] = db
				user := db.FindUser()
				ctx.Global["user"] = user

			}
			func index(ctx *braza.Ctx){
				db := ctx.Global["db"].(database.DB)
				user := ctx.Global["user"].(*User)
				...
			}
	*/
	Global map[string]any

	/*
		Current Cookie Session
		func login(ctx *braza.Ctx){
				db := ctx.Global["db"].(database.DB)
				username,pass,ok := ctx.Request.BasicAuth()
				if ok{
					user := &User{}
					db.Where("username = ?",username).Find(user)
					if user.CompareHashPass(pass){
						ctx.Session.Set("user",user.id)
					}
				}
				ctx.Unauthorized()
			}
	*/
	Session *Session

	/*
		Current Response
			func foo(ctx *braza.Ctx) {
				ctx.JSON(map[string]any{
					"foo":"bar",
				}, 200)
			}
			func foo(ctx *braza.Ctx) {
				ctx.HTML("<h1>Hello</h1>",200)
			}
			func foo(ctx *braza.Ctx) {
				ctx.RenderTemplate("index.html")
		  	}
	*/
	*Response

	/*
		Current Request
	*/
	Request *Request

	/*
		New Schema valid from route schema
			Route{
				Url:"/{foo}/{bar:int}"
				Func: foo,
				Schema: &Schema{}
			}

			type Schema struct {
				Bar int `braza:"in=args"`
				Foo string `braza:"in=args"`
				File *braza.File `braza:"in=files"`
				Files []*braza.File `braza:"in=files"`
				XHeader string `braza:"in=headers"`
				User string `braza:"in=auth,name=username"`
				Pass string `braza:"in=auth,name=password"`
				Limit int `braza:"in=query"` // /path/search?limit=1&offset=2
				Offset int `braza:"in=query"` // /path/search?limit=1&offset=2

				Text string `braza:"in=body"`
				Text2 string  // deafult is 'in=body'.
			}

			func foo(ctx *braza.Ctx) {
				sch := ctx.Schema.(*Schema)
				...
			}
	*/
	Schema        Schema
	SchemaFielder *c3po.Fielder

	/*
		Contains information about the current request, route, etc...

		Can only be accessed if there is a match. otherwise the values ​​will be null
			func xpto(ctx *braza.Ctx) {
				route := ctx.Matchinfo.Route
				router := ctx.Matchinfo.Router
				...
			}
	*/
	MatchInfo *MatchInfo

	mids       []Func
	midCounter int

	backCtx context.Context
}

func (ctx *Ctx) parseMids() {
	ctx.mids = slices.Concat(
		ctx.MatchInfo.Router.Middlewares,
		ctx.MatchInfo.Route.Middlewares,
	)
	ctx.mids = append(ctx.mids, ctx.MatchInfo.Func)
}

// executes the next middleware or main function of the request
func (ctx *Ctx) Next() {

	if ctx.midCounter < len(ctx.mids) {
		n := ctx.mids[ctx.midCounter]
		ctx.midCounter += 1
		n(ctx)
		if ctx.midCounter < len(ctx.mids) {
			ctx.Next()
		}
	}
}

func (ctx *Ctx) UrlFor(name string, external bool, args ...string) string {
	return ctx.App.UrlFor(name, external, args...)
}

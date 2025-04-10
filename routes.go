package braza

import (
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"

	"github.com/5tkgarage/c3po"
)

type Func func(ctx *Ctx)

func (f Func) String() string { return "func(ctx *braza.Ctx)" }

/*
example:

	type Schema struct {
		Bar int `braza:"in=args"`
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

	Route{
		Name:"foo",
		Url:"/{bar:int}",
		Schema: &Schema{},
	}

	func AnyHandler(ctx *braza.Ctx){
		u := ctx.Schema.(*Schema)
		...
	}
*/
type Schema any

type RespSchema map[int]any

type Meth struct {
	Func          Func
	Schema        Schema
	RespSchema    RespSchema
	SchemaFielder *c3po.Fielder
}

type MapCtrl map[string]*Meth

type Route struct {
	/*
		# url patterns
			"" is same "/"
			"/batata"	// match literal "/batata"
			"/{name}"	// match any string (ex: batata1234)
			"/{id:int}"	// match on numbers (ex: 12345)
			"/{path:*}"	|| "/{path:path}" // match anything
			"/{var1:int}/{var2}/{var3}" // is allowed
			`/(\d+)` // regex work too
	*/
	Url string

	// # Route Name
	//	ctx.UrlFor("route Name"...
	Name string

	Func Func

	/*
		allow Cors on this route
		&Cors{
			AllowOrigins: []string{"example.com","www.example.com"},
			AllowMethods: []string{"*","GET","POST"},
			AllowHeaders: []string{"Authorization","*"},
			ExposeHeaders: []string{"X-Header"},
			AllowCredentials: true
		}
	*/
	Cors *Cors

	Schema     Schema
	RespSchema RespSchema

	/*
		# a peculiar way of establishing routes
			Route{
				Name:"foo",
				Url:"/foo",
				MapCtrl: braza.MapCtrl{
					"GET": &braza.Meth{
						Func: Foo,
						Schema: &SchemaFoo{}
					},
					"POST": &braza.Meth{
						Func: Bar,
						Schema: &SchemaBar{}
					},
					"DELETE": &braza.Meth{
						Func: Xpto,
						Schema: &SchemaXpto{}
					},
				},
			}
	*/
	MapCtrl MapCtrl

	// HTTP methods allowed on this route
	//		[]string{"GET","POST","PATCH",...
	Methods []string

	// Func wiil exec before this route
	//		[]Func{	GetUser, HasAUth,...
	Middlewares []Func

	parsed      bool
	router      *Router
	urlRegex    []*regexp.Regexp
	hasSufix    bool
	isStatic    bool
	simpleUrl   string
	simpleName  string
	isUrlPrefix bool
}

func (r *Route) compileUrl() {
	uri := r.Url
	isPrefix := false
	r.hasSufix = strings.HasSuffix(r.simpleUrl, "/")
	if uri != "" && uri != "/" {
		uri = strings.TrimPrefix(strings.TrimSuffix(uri, "/"), "/")
		strs := strings.Split(uri, "/")

		for _, str := range strs {
			if str == "" {
				continue
			}
			if isPrefix {
				log.Panicf("Url Variable Invalid: '%s'", str)
			}
			if re.all.MatchString(str) {
				isPrefix = true
			}
			if re.dot2.MatchString(str) {
				str = re.dot2.ReplaceAllString(str, "/") // -> /../../home = /////home
			}
			if re.slash2.MatchString(str) {
				str = re.slash2.ReplaceAllString(str, "/") // -> /////home = /home
			}

			if re.isVar.MatchString(str) {
				str = re.str.ReplaceAllString(str, `(([\x00-\x7F]+)([^\\\/\s]+)|\d+)`) // This expression will search for non-ASCII values:
				str = re.digit.ReplaceAllString(str, `(\d+)`)
			}
			if !isPrefix {
				r.urlRegex = append(r.urlRegex, regexp.MustCompile(fmt.Sprintf("^%s$", str)))
			}
		}
	}
	if r.hasSufix {
		if r.Url != "/" {
			r.Url = r.Url + "/"
		}
	}
	r.isUrlPrefix = isPrefix
}

func (r *Route) compileMethods() {
	ctrl := MapCtrl{"OPTIONS": &Meth{}}

	// allow Route{URL:"/",Name:"index",Func:func()} with method default "GET"
	if len(r.MapCtrl) == 0 && len(r.Methods) == 0 {
		r.Methods = []string{"GET"}
	}

	for verb, m := range r.MapCtrl {
		v := strings.ToUpper(verb)
		if !reMethods.MatchString(v) {
			l.err.Fatalf("route '%s' has invalid Request Method: '%s'", r.Name, verb)
		}
		if m.Schema != nil {
			sch := c3po.ParseSchemaWithTag("braza", m.Schema)
			m.SchemaFielder = sch
		}
		ctrl[v] = m
	}

	r.MapCtrl = ctrl

	for _, verb := range r.Methods {
		v := strings.ToUpper(verb)
		if !reMethods.MatchString(v) {
			l.err.Fatalf("route '%s' has invalid Request Method: '%s'", r.Name, verb)
		}

		if _, ok := r.MapCtrl[v]; !ok {
			r.MapCtrl[v] = &Meth{
				Func: r.Func,
			}

			if r.Schema != nil {
				sch := c3po.ParseSchemaWithTag("braza", r.Schema)
				r.MapCtrl[v].SchemaFielder = sch
				r.MapCtrl[v].Schema = r.Schema
			}
		}
	}

	r.Methods = []string{}
	for verb := range r.MapCtrl {
		r.Methods = append(r.Methods, verb)
		if verb == "GET" {
			if _, ok := r.MapCtrl["HEAD"]; !ok {
				r.Methods = append(r.Methods, "HEAD")
			}
		}
	}

	if len(r.Methods) <= 1 {
		r.Methods = []string{"GET", "HEAD"}
		r.MapCtrl["GET"] = &Meth{
			Func: r.Func,
		}
	}
}

func (r *Route) parse() {
	if r.Func == nil && r.MapCtrl == nil {
		l.err.Fatalf("Route '%s' need a Func or MapCtrl\n", r.Name)
	}

	r.compileUrl()
	r.compileMethods()
	if r.Cors != nil {
		r.Cors.AllowMethods = r.Methods
	} else {
		r.Cors = &Cors{AllowMethods: r.Methods}
	}
}

func (r *Route) matchURL(ctx *Ctx, url string) bool {
	if url == r.Url {
		return true
	}
	// if url == /  and route.Url == ""
	if url == "/" && r.Url == "" {
		if !ctx.App.StrictSlash {
			return true
		} else {
			return false // poderia fazer um redirect, nas acho q ia dar b.o
		}
	}

	// if url == ""  and route.Url == /
	if url == "" && r.Url == "/" {
		if !ctx.App.StrictSlash {
			return true
		} else {
			return false // poderia fazer um redirect, nas acho q ia dar b.o²
		}
	}

	nurl := strings.TrimPrefix(url, "/")
	nurl = strings.TrimSuffix(nurl, "/")
	urlSplit := strings.Split(nurl, "/")

	lSplit := len(urlSplit)
	lRegex := len(r.urlRegex)

	if lSplit != lRegex {
		if !ctx.App.DisableStatic && r.isStatic {
			if strings.HasPrefix(ctx.Request.URL.Path, ctx.App.StaticUrlPath) {
				return true
			}
		}
		if r.isUrlPrefix {
			if lRegex < lSplit {
				for i, uRe := range r.urlRegex {
					str := urlSplit[i]
					if !uRe.MatchString(str) {
						return false
					}
				}
				return true
			}
		}
		return false
	}

	for i, uRe := range r.urlRegex {
		str := urlSplit[i]
		if !uRe.MatchString(str) {
			return false
		}
	}

	if ctx.App.StrictSlash {
		last := string(url[len(url)-1])
		if r.hasSufix && last == "/" {
			return true
		} else if !r.hasSufix && last != "/" {
			return true
		}
		return false
	}
	return true
}

func (r *Route) match(ctx *Ctx) bool {
	mi := ctx.MatchInfo
	rq := ctx.Request
	m := rq.Method
	url := rq.URL.Path

	if !r.matchURL(ctx, url) {
		return false
	}
	if m == "HEAD" {
		m = "GET"
	}

	if meth, ok := r.MapCtrl[m]; ok {
		mi.MethodNotAllowed = nil
		if meth.Func != nil {
			mi.Func = meth.Func
		}
		mi.Match = true
		mi.Route = r
		ctx.SchemaFielder = meth.SchemaFielder
		return true
	}

	mi.Route = nil
	mi.MethodNotAllowed = ErrorMethodMismatch
	return false
}

func (r *Route) mountURI(args ...string) string {
	var (
		params = map[string]string{}
	)

	c := len(args)
	for i := 0; i < c; i++ {
		if i%2 != 0 {
			continue
		}
		params[args[i]] = args[i+1]
	}

	// Pre Build
	var sUrl = strings.Split(r.Url, "/")
	var urlBuf strings.Builder

	// Build path
	for _, str := range sUrl {
		if re.isVar.MatchString(str) {
			fname := re.getVarName(str)
			value, ok := params[fname]
			if !ok {
				if !re.isVarOpt.MatchString(str) {
					panic(fmt.Errorf("Route '%s' needs parameter '%s' but not passed", r.Name, str))
				}
			} else {
				urlBuf.WriteString("/" + value)
				delete(params, fname)
			}
		} else {
			urlBuf.WriteString("/" + str)
		}
	}
	// Build Query
	var query strings.Builder
	if len(params) > 0 {
		urlBuf.WriteString("?")
		for k, v := range params {
			query.WriteString(k + "=" + v + "&")
		}
	}
	url := urlBuf.String()
	url = re.slash2.ReplaceAllString(url, "/")
	url = re.dot2.ReplaceAllString(url, ".")
	if len(params) > 0 {
		return url + strings.TrimSuffix(query.String(), "&")
	}
	return url
}

func (r *Route) GetRouter() *Router {
	if r.router == nil {
		panic(fmt.Errorf("unregistered route: '%s'", r.Name))
	}
	return r.router
}

/*

 */

func GET(url string, f Func) *Route {
	return &Route{
		Url:     url,
		Func:    f,
		Name:    getFunctionName(f),
		Methods: []string{"GET"},
	}
}

func HEAD(url string, f Func) *Route {
	return &Route{
		Url:     url,
		Func:    f,
		Name:    getFunctionName(f),
		Methods: []string{"HEAD"},
	}
}

func POST(url string, f Func) *Route {
	return &Route{
		Url:     url,
		Func:    f,
		Name:    getFunctionName(f),
		Methods: []string{"POST"},
	}
}

func PUT(url string, f Func) *Route {
	return &Route{
		Url:     url,
		Func:    f,
		Name:    getFunctionName(f),
		Methods: []string{"PUT"},
	}
}

func DELETE(url string, f Func) *Route {
	return &Route{
		Url:     url,
		Func:    f,
		Name:    getFunctionName(f),
		Methods: []string{"DELETE"},
	}
}

func CONNECT(url string, f Func) *Route {
	return &Route{
		Url:     url,
		Func:    f,
		Name:    getFunctionName(f),
		Methods: []string{"CONNECT"},
	}
}

func OPTIONS(url string, f Func) *Route {
	return &Route{
		Url:     url,
		Func:    f,
		Name:    getFunctionName(f),
		Methods: []string{"OPTIONS"},
	}
}

func TRACE(url string, f Func) *Route {
	return &Route{
		Url:     url,
		Func:    f,
		Name:    getFunctionName(f),
		Methods: []string{"TRACE"},
	}
}

func PATCH(url string, f Func) *Route {
	return &Route{
		Url:     url,
		Func:    f,
		Name:    getFunctionName(f),
		Methods: []string{"PATCH"},
	}
}

func Handler(h http.Handler) Func {
	if h == nil {
		panic("handler is nil")
	}
	return func(ctx *Ctx) { h.ServeHTTP(ctx, ctx.Request.raw) }
}

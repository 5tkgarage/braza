package braza

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
)

var (
	allowEnv = map[string]string{
		"":            "development",
		"d":           "development",
		"dev":         "development",
		"development": "development",

		"t":       "test",
		"test":    "test",
		"testing": "test",

		"p":          "production",
		"prod":       "production",
		"production": "production",
	}
	l            = newLogger("")
	listenAll    bool
	localAddress = getOutboundIP()
	mapStackApps = map[string]*App{}
)

var (
	env          string
	port         string
	address      string
	listRoutes   bool
	listRouteSch string
)

func init() {
	flag.StringVar(&env, "env", "", "set a address listener")
	flag.StringVar(&port, "port", "", "set a address listener")
	flag.StringVar(&address, "address", "", "set a address listener")
	flag.BoolVar(&listRoutes, "routes", false, "list all routes")
	flag.StringVar(&listRouteSch, "routeSch", "", "show routes schema-> app.route || app.route:GET")
}

/*
Create a new app with a default settings

	app := NewApp(nil) // or NewApp(*braza.Config{})
	...
	app.Listen()
*/
func NewApp(cfg *Config) *App {
	dfn := ".env"
	if cfg != nil && cfg.DotenvFileName != "" {
		dfn = cfg.DotenvFileName
	}
	godotenv.Load(dfn)
	router := NewRouter("")
	router.main = true
	c := &Config{}
	if cfg != nil {
		*c = *cfg // se estiver clonando o app, evita alguns erros
	}
	app := &App{
		Config:       c,
		Router:       router,
		routers:      []*Router{},
		routerByName: map[string]*Router{},
	}
	return app
}

type App struct {
	// Main Router
	*Router

	// Main Config
	*Config

	/*
		custom auth parser
			app.BasicAuth = (ctx *braza.Ctx) {
				a := ctx.Request.Header.Get("Authorization")
				if a != ""{
					...
					return user,pass, true
				}
				return "","",false
			}
	*/
	BasicAuth func(*Ctx) (user string, pass string, ok bool) // custom func to parse Authorization Header
	/*
		exec after each request (if the application dont crash)
			app.AfterRequest = (ctx *braza.Ctx) {
				h := ctx.Response.Header()
				h.Set("X-Foo","Bar")
				ctx.Response.SetHeader(h)
				...
			}
	*/
	AfterRequest Func

	/*
		exec before each request
			app.BeforeRequest = (ctx *braza.Ctx) {
				db := database.Open()
				ctx.Global["db"] = db
				user,pass,ok := ctx.Request.BasicAuth()
				if ok {
					ctx.Global["user"] = user
				}
			}
	*/
	BeforeRequest Func

	/*
		exec after close each conn ( this dont has effect in response)
			app.TearDownRequest = (ctx *braza.Ctx) {
				database.Close()
				log.Print(...)
				...
			}
	*/
	TearDownRequest Func

	routers      []*Router
	routerByName map[string]*Router

	// The Http.Server
	Srv *http.Server

	uuid  string
	built bool
}

/*
ENV funcs
*/

func (app *App) setFlags() {
	if !flag.Parsed() {
		flag.Parse()
	}

	if e, ok := allowEnv[env]; env != "" && ok {
		app.Env = e
	}
	if app.Srv == nil {
		app.Srv = &http.Server{}
	}
	if address != "" {
		app.Srv.Addr = address
	}
	if port != "" {
		if !re.httpPort.MatchString(port) {
			l.err.Panicf("port '%s' is not valid!", port)
		}
		port = strings.TrimPrefix(port, ":")
		if app.Srv.Addr != "" {
			h, p, err := net.SplitHostPort(app.Srv.Addr)
			if h == "" && p == "" && err != nil {
				l.err.Panic(err)
			}
			app.Srv.Addr = net.JoinHostPort(h, port)
		} else {
			app.Srv.Addr = ":" + port
		}
	}

	if listRoutes {
		app.ShowRoutes()
	}
	if listRouteSch != "" {
		showRouteSchema(app, listRouteSch)
	}
}

func (app *App) setEnv() {
	if e := os.Getenv("ENV"); e != "" {
		app.Env = e
	}
	if s := os.Getenv("SERVERNAME"); s != "" {
		app.Servername = s
	}
	if addr := os.Getenv("ADDRESS"); addr != "" {
		if app.Srv == nil {
			app.Srv = &http.Server{}
		}
		app.Srv.Addr = addr
	}
}

func (app *App) logStarterListener() {
	addr, port, err := net.SplitHostPort(app.Srv.Addr)
	if err != nil {
		l.err.Panic(err)
	}
	envDev := app.Env == "development"
	if listenAll {
		app.Srv.Addr = localAddress
		if envDev {
			l.Defaultf("Server is listening on all address in %sdevelopment mode%s", _RED, _RESET)
		} else {
			l.Default("Server is listening on all address")
		}
		l.info.Printf("          listening on: http://%s:%s", getOutboundIP(), port)
		l.info.Printf("          listening on: http://0.0.0.0:%s", port)
	} else {
		if envDev {
			l.Defaultf("Server is listening in %sdevelopment mode%s", _RED, _RESET)
		} else {
			l.Default("Server is linsten")
		}
		if addr == "" {
			addr = "localhost"
		}
		l.info.Printf("          listening on: http://%s:%s", addr, port)
	}
	if envDev {
		if app.Servername != "" {
			l.info.Printf("          setting the servername to '%s'", app.Servername)
		}
	}
}

/*
SERVER funcs
*/
func (app *App) startListener(c chan error) { c <- app.Srv.ListenAndServe() }

func (app *App) startListenerTLS(privKey, pubKey string, c chan error) {
	c <- app.Srv.ListenAndServeTLS(privKey, pubKey)
}

func (app *App) parseSrvApp(addr string) {
	app.Srv.Handler = app
	app.Srv.MaxHeaderBytes = 1 << 20
	if app.Srv.Addr == "" && addr != "" {
		app.Srv.Addr = addr
	} else if app.Srv.Addr == "" {
		app.Srv.Addr = "localhost:5000"
	}

}

func runSrv(app *App, privKey, pubKey string, host ...string) (err error) {
	envfn := app.DotenvFileName
	if envfn == "" {
		envfn = ".env"
	}
	godotenv.Load(envfn)

	app.Build(host...)

	var reboot = make(chan bool)
	var srvErr = make(chan error)

	if app.Env == "development" && !app.DisableFileWatcher {
		go fileWatcher(reboot)
	}

	if !app.Silent {
		app.logStarterListener()
	}

	if privKey != "" && pubKey != "" {
		go app.startListenerTLS(privKey, pubKey, srvErr)
	} else {
		go app.startListener(srvErr)
	}

	if !app.DisableFileWatcher {
		for {
			select {
			case <-reboot:
				app.Srv.Close()
				selfReboot()
			case err = <-srvErr:
				if !errors.Is(err, http.ErrServerClosed) {
					l.err.Println(err)
					return err
				}
			}
		}
	} else {
		e := <-srvErr
		l.err.Println(e)
		return e
	}
}

// Start Listener in http
func (app *App) Listen(host ...string) (err error) { return runSrv(app, "", "", host...) }

// Start Listener in https
func (app *App) ListenTLS(certFile, certKey string, host ...string) (err error) {
	return runSrv(app, certFile, certKey, host...)
}

/*
APP methods
*/

// Parse the router and your routes
func (app *App) parseApp() {
	app.uuid = uuid.NewString()
	app.setEnv()
	app.checkConfig()
	if app.Servername != "" {

		srv := strings.TrimPrefix(
			strings.TrimPrefix(
				app.Servername, "https//",
			),
			"https://",
		) // so pra evitar erros

		srv = strings.TrimPrefix(
			strings.TrimSuffix(
				srv, "/",
			), ".") // so pra evitar erros²

		h, p, err := net.SplitHostPort(srv)
		if err != nil && p != "" && h != "" {
			log.Fatal(err)
		}
		if p != "" {
			app.serverport = p
		}
		if h != "" {
			app.Servername = h
		}
	}

	if env, ok := allowEnv[app.Env]; ok {
		app.Env = env
	} else {
		l.err.Fatalf("environment '%s' is not valid", app.Env)
	}

	if !app.DisableStatic {
		staticUrl := "/assets"
		fp := "/{filepath:path}"
		if app.StaticUrlPath != "" {
			staticUrl = app.StaticUrlPath
		}
		path := filepath.Join(staticUrl, fp)
		app.AddRoute(&Route{
			Url:      path,
			Func:     serveFileHandler,
			Name:     "assets",
			isStatic: true,
		})
	}

	// se o usuario mudar o router principal, isso evita alguns erro
	if !app.main {
		app.main = true

		if app.Router.Routes == nil {
			app.Router.Routes = []*Route{}
		}
		if app.Router.routesByName == nil {
			app.Router.routesByName = map[string]*Route{}
		}
		if app.Router.Cors == nil {
			app.Router.Cors = &Cors{}
		}
		if app.Router.Middlewares == nil {
			app.Router.Middlewares = []Func{}
		}

	}

	if !slices.Contains(app.routers, app.Router) {
		app.routers = append(app.routers, app.Router)
	}
	for _, router := range app.routers {
		router.parse(app.Servername)
		for n, r := range router.routesByName {
			app.routesByName[n] = r
		}
	}
	app.built = true
}

/*
Custom Http Error Handler

	app.ErrorHandler(401,(ctx *braza.Ctx) {
		ctx.HTML("Access denied",401)
	})
	app.ErrorHandler(404,(ctx *braza.Ctx) {
		ctx.HTML("Hey boy, you're a little lost",404)
	})
*/
func (app *App) ErrorHandler(statusCode int, f Func) {
	if app.errHandlers == nil {
		app.errHandlers = map[int]Func{}
	}
	app.errHandlers[statusCode] = f
}

/*
Register Router in app

	func main() {
		api := braza.NewRouter("api")
		api.post("/products")
		api.get("/products/{productID:int}")

		app := braza.NewApp(nil)
		app.Mount(api)
		// do anything ...
		app.Listen()
	}
*/
func (app *App) Mount(routers ...*Router) {
	for _, router := range routers {
		if router.Name == "" {
			panic(fmt.Errorf("the routers must be named"))
		} else if _, ok := app.routerByName[router.Name]; ok {
			panic(fmt.Errorf("router '%s' already regitered", router.Name))
		}
		if !slices.Contains(app.routers, app.Router) {
			app.routers = append(app.routers, router)
		}
	}
}

/*
Build the App, but not start serve

example:

	func index(ctx braza.Ctx){}

	// it's work
	func main() {
		app := braza.NewApp()
		app.GET("/",index)
		app.Build()
		app.UrlFor("index",true)
	}
	// it's don't work
	func main() {
		app := braza.NewApp()
		app.GET("/",index)
		app.UrlFor("index",true)
	}
*/
func (app *App) Build(addr ...string) {
	if app.built {
		return
	}
	app.parseApp()
	l = newLogger(app.LogFile)

	var address = ":5000"
	if len(addr) > 0 {
		a_ := addr[0]
		if a_ != "" {
			_, _, err := net.SplitHostPort(a_)
			if err == nil {
				address = a_
			}
		}
	}

	if strings.Contains(address, "0.0.0.0") {
		listenAll = true
	}
	app.setFlags()
	app.parseSrvApp(address)
}

/*
HTTP funcs
*/

func (app *App) match(ctx *Ctx) {
	rq := ctx.Request
	if app.Servername != "" {
		if net.ParseIP(rq.Host) != nil {
			ctx.NotFound()
		}

		if !strings.Contains(rq.Host, app.Servername) {
			ctx.NotFound()
		}
	}

	for _, router := range app.routers {
		if router.match(ctx) {
			if router.StrictSlash && !strings.HasSuffix(rq.URL.Path, "/") {
				args := []string{}
				for k, v := range rq.PathArgs {
					args = append(args, k, v)
				}
				ctx.Response.Redirect(ctx.UrlFor(ctx.MatchInfo.Route.Name, true, args...))
			}
			return
		}
	}
	mi := ctx.MatchInfo
	if mi.MethodNotAllowed != nil {
		ctx.MethodNotAllowed()
	}
	ctx.NotFound()
}

// exec route and handle errors of application
func (app *App) execRoute(ctx *Ctx) {
	app.match(ctx)

	rq := ctx.Request
	mi := ctx.MatchInfo

	rq.parse()
	ctx.parseMids()

	if app.BeforeRequest != nil {
		app.BeforeRequest(ctx)
	}

	if mi.Func == nil && rq.Method == "OPTIONS" {
		optionsHandler(ctx)
		return
	}
	ctx.Next()
}

func (app *App) execHandlerError(ctx *Ctx, code int) {
	ctx.Reset()
	if h, ok := app.errHandlers[code]; ok {
		ctx.StatusCode = code
		h(ctx)
	} else {
		ctx.StatusCode = code
		statusText := http.StatusText(code)
		body := fmt.Sprintf("%d %s", code, statusText)
		ctx.header.Set("Content-Type", "text/plain")
		ctx.WriteString(body)
	}
}

func (app *App) closeConn(ctx *Ctx) {
	err := recover()
	defer execTeardown(ctx)
	defer req500(ctx)

	rsp := ctx.Response
	if err == nil {
		reqOK(ctx)
		return
	}
	if e, ok := err.(error); ok && errors.Is(ErrHttpAbort, e) {
		code := ctx.backCtx.Value(abortCode(1))
		if c, ok := code.(int); ok {
			app.execHandlerError(ctx, c)
		}
		reqOK(ctx)
	} else {
		rsp.StatusCode = 500
		statusText := "500 Internal Server Error"
		l.Error(err)
		rsp.raw.WriteHeader(500)
		fmt.Fprint(rsp.raw, statusText)
	}
}

// Url Builder
//
//	app.GET("/users/{userID:int}", index)
//
//	app.UrlFor("index", false, "userID", "1"}) //  /users/1
//	app.UrlFor("index", true, "userID", "1"}) // http://servername/users/1
func (app *App) UrlFor(name string, external bool, args ...string) string {
	var (
		host   = ""
		route  *Route
		router *Router
	)
	if !app.built {
		l.err.Fatalf("you are trying to use this function outside of a context")
	}
	if len(args)%2 != 0 {
		l.err.Fatalf("numer of args of build url, is invalid: UrlFor only accept pairs of args ")
	}

	// check route name
	if r, ok := app.routesByName[name]; ok {
		route = r
	} else {
		panic(fmt.Sprintf("Route '%s' is undefined \n", name))
	}
	router = route.router

	params := map[string]string{}
	c := len(args)
	for i := 0; i < c; i++ {
		if i%2 != 0 {
			continue
		}
		params[args[i]] = args[i+1]
	}

	// Build Host
	if external {
		schema := "http://"
		if app.ListeningInTLS {
			schema = "https://"
		}
		srvname := app.Servername
		if app.serverport != "" && (app.serverport != "80" && app.serverport != "443") {
			srvname = net.JoinHostPort(app.Servername, app.serverport)
		}
		if router.Subdomain != "" {
			host = schema + router.Subdomain + "." + srvname
		} else {
			if app.Servername == "" {
				_, p, _ := net.SplitHostPort(app.Srv.Addr)
				h := net.JoinHostPort(localAddress, p)
				host = schema + h
			} else {
				host = schema + srvname
			}
		}
	}
	url := route.mountURI(args...)
	return host + url
}

// http.Handler
func (app *App) ServeHTTP(wr http.ResponseWriter, req *http.Request) {
	ctx := NewCtx(app, wr, req)
	defer app.closeConn(ctx)
	app.execRoute(ctx)

}

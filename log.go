package braza

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"sort"
	"strings"

	"5tk.dev/c3po"
)

const (
	_RED            = "\033[31m"
	_BLUE           = "\033[34m"
	_CYAN           = "\033[36m"
	_BLACK          = "\033[30m"
	_GREEN          = "\033[32m"
	_WHITE          = "\033[37m"
	_YELLOW         = "\033[33m"
	_MAGENTA        = "\033[35m"
	_BRIGHT_RED     = "\033[91m"
	_BRIGHT_BLUE    = "\033[94m"
	_BRIGHT_CYAN    = "\033[96m"
	_BRIGHT_BLACK   = "\033[90m"
	_BRIGHT_GREEN   = "\033[92m"
	_BRIGHT_WHITE   = "\033[97m"
	_BRIGHT_YELLOW  = "\033[93m"
	_BRIGHT_MAGENTA = "\033[95m"
	_RESET          = "\033[m"
)

func newLogger(logFile string) *logger {
	var lFile *log.Logger
	if logFile != "" {
		var f *os.File
		_, err := os.Stat(logFile)
		if err != nil {
			f, err = os.Create(logFile)
			if err != nil {
				panic(err)
			}
		} else {
			f, err = os.Open(logFile)
			if err != nil {
				panic(err)
			}
		}
		lFile = log.New(f, "", log.Ldate|log.Ltime)
	}

	return &logger{
		err:      log.New(os.Stdout, _RED+"error: "+_RESET, 0),
		warn:     log.New(os.Stdout, _YELLOW+"warn: "+_RESET, log.Ldate|log.Ltime),
		info:     log.New(os.Stdout, _GREEN+"info: "+_RESET, log.Ldate|log.Ltime),
		logFile:  lFile,
		prodInfo: log.New(os.Stdout, "", log.Ldate|log.Ltime),
	}
}

type logger struct {
	prodInfo *log.Logger
	info     *log.Logger
	warn     *log.Logger
	err      *log.Logger
	logFile  *log.Logger
}

func (l *logger) Default(v ...any) {
	l.info.Println(v...)
	if l.logFile != nil {
		l.logFile.Println(v...)
	}
}

func (l *logger) Defaultf(formatString string, v ...any) {
	l.info.Printf(formatString, v...)
	if l.logFile != nil {
		l.logFile.Printf(formatString, v...)
	}
}

func (l *logger) Error(v ...any) {
	l.err.Println(v...)
	if l.logFile != nil {
		l.logFile.Println(v...)
	}
	for i := 0; i < 20; i++ {
		_, file, line, ok := runtime.Caller(i)
		if !ok {
			return
		}
		fmt.Printf("\t%s:%d\n", file, line)
	}
}

func (l *logger) LogRequest(ctx *Ctx) {
	rq := ctx.Request
	rsp := ctx.Response

	color := ""
	switch {
	case rsp.StatusCode >= 500:
		color = _RED
	case rsp.StatusCode >= 400:
		color = _YELLOW
	case rsp.StatusCode >= 300:
		color = _CYAN
	case rsp.StatusCode >= 200:
		color = _GREEN
	default:
		color = _WHITE
	}
	addr := ""
	if ctx.MatchInfo.Router != nil {
		addr = ctx.MatchInfo.Router.Subdomain
	}
	if addr != "" {
		addr = addr + ".[...]" + rq.URL.Path
	} else {
		addr = rq.URL.Path
	}

	if l.logFile != nil {
		l.logFile.Printf("%d -> %s -> %s", rsp.StatusCode, rq.Method, addr)
	}
	if ctx.App.Silent {
		return
	}

	appName := ctx.App.Srv.Addr
	if ctx.App.Name != "" {
		appName = ctx.App.Name + " > " + appName
	}
	if ctx.App.Env == "production" {
		l.prodInfo.Printf("%d -> %s -> %s", rsp.StatusCode, rq.Method, addr)
	} else {
		l.info.Printf("%s %s%d%s -> %s -> %s", appName, color, rsp.StatusCode, _RESET, rq.Method, addr)
	}
}

func (app *App) ShowRoutes() {
	if !app.built {
		app.Build()
	}
	nameLen := 0
	methLen := 0
	pathLen := 0
	subDoLen := 0

	listRouteName := []string{}
	for _, r := range app.routesByName {
		listRouteName = append(listRouteName, r.Name)
		if nl := len(r.Name); nl > nameLen {
			nameLen = nl
		}
		if ml := len(strings.Join(r.Methods, ",")); ml > methLen {
			methLen = ml
		}
		if pl := len(r.Url); pl > pathLen {
			pathLen = pl
		}
		if r.router.Subdomain != "" {
			router := r.router
			// if router != nil && router.Subdomain != "" {
			// }
			if l := len(router.Subdomain); l > subDoLen {
				subDoLen = l
			}
		}
	}
	sort.Strings(listRouteName)

	if nameLen < 6 {
		nameLen = 6
	}
	if methLen < 7 {
		methLen = 7
	}
	if pathLen < 9 {
		pathLen = 9
	}
	if subDoLen < 10 && subDoLen != 0 {
		subDoLen = 10
	}

	line1 := strings.Repeat("-", nameLen+1)
	line2 := strings.Repeat("-", methLen+1)
	line3 := strings.Repeat("-", pathLen+1)
	line4 := strings.Repeat("-", subDoLen+1)

	routeN := "ROUTES " + strings.Repeat(" ", nameLen-6)
	methodsN := "METHODS " + strings.Repeat(" ", methLen-7)
	endpointN := "ENDPOINTS " + strings.Repeat(" ", pathLen-9)

	if subDoLen > 0 {
		subdomainN := "SUBDOMAINS " + strings.Repeat(" ", subDoLen-10)

		fmt.Printf("+-%s+-%s+-%s+-%s+\n", line1, line2, line3, line4)
		fmt.Printf("| %s| %s| %s| %s|\n", routeN, methodsN, endpointN, subdomainN)
		fmt.Printf("+-%s+-%s+-%s+-%s+\n", line1, line2, line3, line4)
		for _, rName := range listRouteName {
			r := app.routesByName[rName]
			mths_ := strings.Join(r.Methods, ",")
			space1 := nameLen - len(rName)
			space2 := methLen - len(mths_)
			space3 := pathLen - len(r.Url)
			space4 := subDoLen - len(r.GetRouter().Subdomain)

			endpoint := r.Name + strings.Repeat(" ", space1)
			mths := mths_ + strings.Repeat(" ", space2)
			path := r.Url + strings.Repeat(" ", space3)
			sub := r.GetRouter().Subdomain + strings.Repeat(" ", space4)
			fmt.Printf("| %s | %s | %s | %s |\n", endpoint, mths, path, sub)
		}
		fmt.Printf("+-%s+-%s+-%s+-%s+\n", line1, line2, line3, line4)
	} else {
		fmt.Printf("+-%s+-%s+-%s+\n", line1, line2, line3)
		fmt.Printf("| %s| %s| %s|\n", routeN, methodsN, endpointN)
		fmt.Printf("+-%s+-%s+-%s+\n", line1, line2, line3)
		for _, rName := range listRouteName {
			r := app.routesByName[rName]
			mths_ := strings.Join(r.Methods, ",")
			space1 := nameLen - len(rName)
			space2 := methLen - len(mths_)
			space3 := pathLen - len(r.Url)

			endpoint := r.Name + strings.Repeat(" ", space1)
			mths := mths_ + strings.Repeat(" ", space2)
			path := r.Url + strings.Repeat(" ", space3)
			fmt.Printf("| %s | %s | %s |\n", endpoint, mths, path)
		}
		fmt.Printf("+-%s+-%s+-%s+\n", line1, line2, line3)
	}
}

func showRouteSchema(app *App, routeName string) {
	var rname, meth = routeName, ""

	parts := strings.Split(routeName, ":")
	if len(parts) > 2 {
		panic("invalid route name")
	} else if len(parts) == 2 {
		rname = parts[0]
		meth = strings.ToUpper(parts[1])
	} else {
		panic("routeSchema flag need a method. example: api.setProducts:POST")
	}
	if r, ok := app.routesByName[rname]; ok {
		if m, ok := r.MapCtrl[meth]; ok {
			if m.SchemaFielder != nil {
				fmt.Println("Request Schema:")
				fmt.Println("\t", m.SchemaFielder)
			}
			if len(m.RespSchema) > 0 {
				fmt.Println("Response Schema:")
				for c, r := range m.RespSchema {
					sch := c3po.ParseSchemaWithTag("braza", r)
					fmt.Printf("%d: %s\n", c, sch)
					fmt.Println()
				}
			}
		}
	}

}

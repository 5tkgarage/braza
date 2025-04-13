package braza

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"mime"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"5tk.dev/c3po"
)

var (
	htmlTemplates sync.Map
)

func NewResponse(wr http.ResponseWriter, ctx *Ctx) *Response {
	return &Response{
		Buffer:     bytes.NewBufferString(""),
		raw:        wr,
		ctx:        ctx,
		header:     http.Header{},
		StatusCode: 200,
	}
}

type Response struct {
	*bytes.Buffer
	header     http.Header
	StatusCode int
	ctx        *Ctx
	raw        http.ResponseWriter
}

func (r Response) SetHeader(h http.Header)        { r.header = h }
func (r Response) Header() http.Header            { return r.header }
func (r Response) Write(b []byte) (int, error)    { return r.Buffer.Write(b) }
func (r Response) WriteHeader(statusCode int)     { r.StatusCode = statusCode }
func (r *Response) SetCookie(cookie *http.Cookie) { SetCookie(r.header, cookie) }
func (r *Response) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return r.raw.(http.Hijacker).Hijack()
}

func (r *Response) writeAny(body any) {
	switch v := body.(type) {
	default:
		r.WriteString(fmt.Sprint(v))
	case string:
		r.WriteString(v)
	case []byte:
		r.Write(v)
	case io.Reader:
		io.Copy(r, v)
	}
}

func (r *Response) parseHeaders() {
	ctx := r.ctx
	method := ctx.Request.Method
	routerCors := ctx.MatchInfo.Router.Cors
	if routerCors != nil {
		routerCors.parse(r.header, ctx.Request)
	}
	routeCors := ctx.MatchInfo.Route.Cors

	h := r.header
	if routeCors != nil {
		routeCors.parse(r.header, ctx.Request)
	}
	if routerCors != nil || routeCors != nil {
		if _, ok := h["Access-Control-Request-Method"]; !ok {
			h.Set("Access-Control-Request-Method", method)
		}
	}
	if method == "OPTIONS" {
		if _, ok := h["Access-Control-Allow-Origin"]; !ok {
			h.Set("Access-Control-Allow-Origin", ctx.App.Servername)
		}
		if _, ok := h["Access-Control-Allow-Headers"]; !ok {
			h.Set("Access-Control-Allow-Headers", "content-type")
		}
		if _, ok := h["Access-Control-Expose-Headers"]; !ok {
			h.Set("Access-Control-Expose-Headers", "")
		}
		if _, ok := h["Access-Control-Allow-Credentials"]; !ok {
			h.Set("Access-Control-Allow-Credentials", "false")
		}
	} else if ctx.Request.Method == "HEAD" {
		r.Reset()
	}
}

func (r *Response) textCode(code int) {
	statusText := http.StatusText(code)
	if statusText == "" {
		panic(fmt.Errorf("unknown status code:'%d'", code))
	}
	r.ctx.backCtx = context.WithValue(r.ctx.backCtx, abortCode(1), code)
	panic(ErrHttpAbort)
}

// Redirect to Following URL
func (r *Response) Redirect(url string) {
	r.Reset()
	r.header.Set("Location", url)
	r.StatusCode = 302

	r.header.Set("Content-Type", "text/html; charset=utf-8")
	r.WriteString("<a href=\"" + c3po.HtmlEscape(url) + "\"> Manual Redirect </a>.\n")
	panic(ErrHttpAbort)
}

func (r *Response) JSON(body any, code int) {
	r.Reset()
	r.StatusCode = code
	r.header.Set("Content-Type", "application/json")

	if b, ok := body.(string); ok {
		r.WriteString(b)
		panic(ErrHttpAbort)
	} else if b, ok := body.(error); ok {
		r.WriteString(b.Error())
		panic(ErrHttpAbort)
	} else if b, ok := body.(Jsonify); ok {
		j, err := json.Marshal(b.ToJson())
		if err != nil {
			panic(err)
		}
		r.Write(j)
		panic(ErrHttpAbort)
	}

	j, err := json.Marshal(body)
	if err != nil {
		panic(err)
	}
	r.Write(j)

	panic(ErrHttpAbort)
}

func (r *Response) TEXT(body any, code int) {
	r.Reset()
	r.StatusCode = code
	r.header.Set("Content-Type", "text/plain")
	r.writeAny(body)
	panic(ErrHttpAbort)
}

func (r *Response) HTML(body any, code int) {
	r.Reset()
	r.StatusCode = code
	r.header.Set("Content-Type", "text/html")
	r.writeAny(body)
	panic(ErrHttpAbort)
}

func (r *Response) Abort(code int)       { r.textCode(code) }
func (r *Response) Close()               { panic(ErrHttpAbort) }
func (r *Response) Ok()                  { r.textCode(200) }
func (r *Response) Created()             { r.textCode(201) }
func (r *Response) NoContent()           { r.textCode(204) }
func (r *Response) BadRequest()          { r.textCode(400) }
func (r *Response) Unauthorized()        { r.textCode(401) }
func (r *Response) Forbidden()           { r.textCode(403) }
func (r *Response) NotFound()            { r.textCode(404) }
func (r *Response) MethodNotAllowed()    { r.textCode(405) }
func (r *Response) ImATaerpot()          { r.textCode(418) }
func (r *Response) InternalServerError() { r.textCode(500) }

func (r *Response) RenderTemplate(tmpl string, data ...any) {
	var (
		t       *template.Template
		_t      any
		ok      bool
		value   any
		app     = r.ctx.App
		lenFile int
	)

	if _t, ok = htmlTemplates.Load(app.Name + ":" + tmpl); !ok || (app.Env == "development" && !app.DisableTemplateReloader) {
		pa := filepath.Join(app.TemplateFolder, tmpl)
		_, err := os.Stat(pa)
		if err != nil {
			if app.Env == "development" {
				r.TEXT(err, 404)
			}
			r.NotFound()
		}
		f, err := os.ReadFile(pa)
		r.CheckErr(err)

		t, err = template.New(tmpl).
			Funcs(r.ctx.App.TemplateFuncs).
			Parse(string(f))
		r.CheckErr(err)
		lenFile = len(f)
		htmlTemplates.Store(r.ctx.App.Name+":"+tmpl, t)
	} else {
		t = _t.(*template.Template)
	}
	if len(data) == 1 {
		value = data[0]
	}
	t.Execute(r, value)
	if r.Buffer.Len() == 0 && lenFile > 0 {
		r.TEXT("ouve um erro durante o parse do html, por favor, verificar o arquivo", 500)
	}
	r.Close()
}

// if err != nil, return a 500 Intenal Server Error
func (r *Response) CheckErr(err error) {
	if err != nil {
		l.err.Println(err)
		if r.ctx.App.Env == "development" {
			r.TEXT(err, 500)
		}
		r.InternalServerError()
	}
}

// Serve File
func (r *Response) ServeFile(pathToFile string) {
	ctx := r.ctx
	f, err := os.Open(pathToFile)
	if err == nil {
		_, file := filepath.Split(pathToFile)
		defer f.Close()
		if fStat, err := f.Stat(); err != nil || fStat.IsDir() {
			ctx.NotFound()
		}
		ctx.ReadFrom(f)
		ctype := mime.TypeByExtension(filepath.Ext(file))
		if ctype == "application/octet-stream" {
			ctype = http.DetectContentType(ctx.Bytes())
		}
		ctx.header.Set("Content-Type", ctype)
		ctx.Close()
	}
	if ctx.App.Env == "development" {
		ctx.TEXT(err, 404)
	}
	ctx.Response.NotFound()
}

package braza

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"5tk.dev/c3po"
)

type reqSchema struct {
	value  any
	errors []error
}

func (q *reqSchema) Value() any      { return q.value }
func (q *reqSchema) Errors() []error { return q.errors }
func (q *reqSchema) HasErrors() bool { return len(q.errors) > 0 }

func MountSchemaFromRequest(f *c3po.Fielder, rq *Request) c3po.Schema {
	var sch any
	var err error
	var errs = []error{}

	switch f.Type {
	default:
		v, isFile := getData(f, rq)
		schT := reflect.TypeOf(f.Schema)
		if schT.Kind() == reflect.Ptr {
			schT = schT.Elem()
		}
		if v == nil || v == "" {
			if f.Required {
				return &reqSchema{
					errors: []error{
						c3po.RetMissing(f),
					},
				}
			}
			sch = reflect.New(schT).Elem()
			break
		}
		if isFile {
			if f.IsSlice {
				return &reqSchema{value: v}
			}
			return &reqSchema{value: v.([]*File)[0]}
		}
		schN := reflect.New(schT).Elem()
		schV := reflect.ValueOf(v)
		if !c3po.SetReflectValue(schN, schV) {
			return &reqSchema{errors: []error{c3po.RetInvalidType(f)}}
		}
		return &reqSchema{value: schN}
	case reflect.Slice:
		v, isFile := getData(f, rq)
		if v == nil {
			sch = reflect.MakeSlice(reflect.SliceOf(reflect.TypeOf(f.SliceType.Schema)), 0, 0)
			if f.Required {
				return &reqSchema{errors: []error{c3po.RetMissing(f)}}
			}
			return &reqSchema{value: sch.(reflect.Value).Interface()}
		}
		if isFile {
			return &reqSchema{value: v}
		} else {
			sch = f.Decode(v)
		}
		if sch.(c3po.Schema).HasErrors() {
			errs = append(errs, err)
		}
	case reflect.Struct:
		var rt reflect.Type
		if f.IsPointer {
			rt = reflect.TypeOf(f.Schema).Elem()
		} else {
			rt = reflect.TypeOf(f.Schema)
		}
		_sch := reflect.New(rt).Elem()
		for i := 0; i < rt.NumField(); i++ {
			fName := f.FieldsByIndex[i]
			fielder := f.Children[fName]
			rtField := _sch.Field(i)

			v, isFile := getData(fielder, rq)
			if v == nil {
				if fielder.Required {
					errs = append(errs, c3po.RetMissing(fielder))
				}
				continue
			}
			if isFile {
				if !fielder.IsSlice {
					_v, ok := v.([]*File)
					if ok && len(_v) > 0 {
						v = _v[0]
					} else {
						if fielder.Required {
							errs = append(errs, c3po.RetMissing(fielder))
						}
						continue
					}
				}
				if !c3po.SetReflectValue(rtField, reflect.ValueOf(v)) {
					errs = append(errs, c3po.RetInvalidType(fielder))
				}
			} else {
				schF := fielder.Decode(v)

				if schF.HasErrors() {
					errs = append(errs, schF.Errors()...)
				} else {
					if !c3po.SetReflectValue(rtField, reflect.ValueOf(schF)) {
						errs = append(errs, c3po.RetInvalidType(fielder))
					}
				}
			}
		}
		if len(errs) == 0 {
			sch = _sch
		}
	}
	if len(errs) > 0 {
		if f.Name != "" {
			return &reqSchema{
				errors: []error{fmt.Errorf(`{"%s":%v}`, f.Name, formatErr(errs...))},
			}
		}
		return &reqSchema{errors: []error{formatErr(errs...)}}
	}
	return &reqSchema{value: sch}
}

func formatErr(errs ...error) error {
	if len(errs) > 1 {
		errBuf := bytes.NewBufferString("[")
		for i, e := range errs {
			if i > 0 {
				errBuf.WriteString(",")
			}
			errBuf.WriteString(e.Error())
		}
		errBuf.WriteString("]")
		return errors.New(errBuf.String())
	}
	if len(errs) == 1 {
		e := errs[0]
		if e != nil {
			return e
		}
	}
	return nil
}

// return nil if not exists and a bool if is a file
func getData(f *c3po.Fielder, rq *Request) (any, bool) {
	fName := f.Name
	if fName == "" {
		fName = f.RealName
	}
	if fName == "" {
		return nil, false
	}
	isFile := false
	if _, ok := f.Schema.([]*File); ok {
		isFile = true
	}
	if _, ok := f.Schema.(*File); ok {
		isFile = true
	}

	switch strings.ToLower(f.Tags["in"]) {
	default:
		if isFile {
			return rq.Files[f.Name], true
		}
		return rq.Form[f.Name], false
	case "body":
		return rq.Form[f.Name], false
	case "path":
		return rq.PathArgs[f.Name], false
	case "subdomain":
		return rq.PathArgs[f.Name], false
	case "files":
		return rq.Files[f.Name], true
	case "headers":
		return rq.Header.Get(f.Name), false
	case "query":
		return rq.Query.Get(f.Name), false
	case "auth":
		if u, p, ok := rq.BasicAuth(); ok {
			if f.Name == "username" {
				return u, false
			} else if f.Name == "password" {
				return p, false
			}
		}
	}
	return nil, false
}

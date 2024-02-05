package problems

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"reflect"
	"strings"
)

type Problem interface {
	Problem() (typ string, title string, status int, detail string, instance string, data map[string]any)
}

func ServeProblem(resp http.ResponseWriter, p Problem) {
	typ, title, status, detail, instance, data := p.Problem()

	statusCode := status
	for statusCode > 1000 {
		statusCode /= 10
	}

	if typ == "" {
		typ = "about:blank"
	}

	resp.Header().Set("X-Content-Type-Options", "nosniff")
	resp.Header().Set("Content-Type", "application/problem+json; charset=utf-8")
	resp.WriteHeader(statusCode)

	encoder := json.NewEncoder(resp)
	encoder.SetIndent("", "  ")
	err := encoder.Encode(problem{
		typ:      typ,
		title:    title,
		status:   status,
		detail:   detail,
		instance: instance,
		data:     data,
	})
	if err != nil {
		log.Printf("problem: can not marshal problem as json: %v, error: %v", p, err)
	}
}

type problem struct {
	typ      string
	title    string
	status   int
	detail   string
	instance string
	data     map[string]any
}

func (p problem) MarshalJSON() ([]byte, error) {
	m := make(map[string]any)
	m["type"] = p.typ
	m["title"] = p.title
	m["status"] = p.status
	if p.detail != "" {
		m["detail"] = p.detail
	}
	if p.instance != "" {
		m["instance"] = p.instance
	}
	for k, v := range p.data {
		m[k] = v
	}
	return json.Marshal(m)
}

func Pprintf(format string, args ...any) (str string, data map[string]any, wraps error) {
	n := numArgsPattern(format)
	if len(args) < n {
		panic(fmt.Sprintf("pattern has %d args for %d placeholders", len(args), n))
	}
	if n != 0 {
		str = fmt.Sprintf(format, args[:n]...)
		args = args[n:]
	} else {
		str = format
	}
	if len(args) > 0 {
		var ok bool
		wraps, ok = args[len(args)-1].(error)
		if ok {
			args = args[:len(args)-1]
		}
	}
	data = denseArgs(args...)
	return str, data, wraps
}

func numArgsPattern(s string) int {
	n := 0
	for {
		i := strings.IndexByte(s, '%')
		if i == -1 {
			return n
		}
		s = s[i+1:]
		if len(s) > 0 && s[0] == '%' {
			s = s[1:]
		} else {
			n++
		}
	}
}

func denseArgs(args ...any) map[string]any {

	switch len(args) {
	case 0:
		return nil
	case 1:
		if m, ok := args[0].(map[string]any); ok {
			return m
		}
		r := reflect.ValueOf(args[0])
		for r.Kind() == reflect.Ptr {
			r = r.Elem()
		}
		if r.Kind() != reflect.Struct {
			panic(fmt.Errorf("invalid key[0]: unsupported type %t", args[0]))
		}
		m := make(map[string]any)
		for j := 0; j < r.NumField(); j++ {
			f := r.Field(j)
			if f.CanInterface() {
				m[r.Type().Field(j).Name] = f.Interface()
			}
		}
		return m
	default:
		m := make(map[string]any)
		if len(args)%2 != 0 {
			panic(errors.New("missing unmatched key-value pairs"))
		}
		for i := 0; i < len(args); i += 2 {
			if key, ok := args[i].(string); ok {
				m[key] = args[i+1]
			} else {
				panic(fmt.Errorf("invalid key[%d]: unsupported type %t", i, args[i]))
			}
		}
		return m
	}
}

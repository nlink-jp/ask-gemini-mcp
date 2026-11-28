package tools

import (
	"reflect"
	"strings"
)

// argNames reports whether a name is an argument the decode target declares,
// spelt as declared. A field with an explicit json name accepts that name
// exactly; a field without one is matched as encoding/json matches it (any
// case), since no spelling is declared for it; an embedded struct's fields
// count as the target's own; "-" fields are not arguments. A target that is
// not a struct (a map, say) declares no names, so every name passes here and
// is left to the decoder.
func argNames(into any) func(string) bool {
	t := reflect.TypeOf(into)
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t == nil || t.Kind() != reflect.Struct {
		return func(string) bool { return true }
	}
	exact := map[string]bool{}
	var loose []string
	var walk func(reflect.Type)
	walk = func(t reflect.Type) {
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			tag := f.Tag.Get("json")
			if tag == "-" {
				continue
			}
			name, _, _ := strings.Cut(tag, ",")
			if f.Anonymous && name == "" {
				ft := f.Type
				for ft.Kind() == reflect.Pointer {
					ft = ft.Elem()
				}
				if ft.Kind() == reflect.Struct {
					walk(ft)
					continue
				}
			}
			if !f.IsExported() {
				continue
			}
			if name == "" {
				loose = append(loose, f.Name)
			} else {
				exact[name] = true
			}
		}
	}
	walk(t)
	return func(n string) bool {
		if exact[n] {
			return true
		}
		for _, l := range loose {
			if strings.EqualFold(l, n) {
				return true
			}
		}
		return false
	}
}

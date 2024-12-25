package shared

import (
	"fmt"
	"net/http"
	"reflect"
)

func PrintTypes(s interface{}) {
	v := reflect.ValueOf(s)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fmt.Printf("Field %s: %s\n", t.Field(i).Name, field.Type())
	}
}

func ConvertHeaders(headers http.Header) map[string]string {
	converted := make(map[string]string)
	for key, values := range headers {
		if len(values) > 0 {
			converted[key] = values[0]
		}
	}
	return converted
}

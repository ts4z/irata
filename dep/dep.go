/*
package dep provides utilities for dependency injection.

okay, just the one.
*/
package dep

import (
	"fmt"
	"reflect"
	"runtime"
)

func Required[T any](t T) T {
	if reflect.ValueOf(t).IsValid() {
		return t
	} else {
		// Get caller information
		pc, file, line, ok := runtime.Caller(1)
		if !ok {
			panic(fmt.Sprintf("missing required dependency of type %T", t))
		}
		fn := runtime.FuncForPC(pc)
		if fn != nil {
			panic(fmt.Sprintf("missing required dependency in %s (%s:%d)", fn.Name(), file, line))
		} else {
			panic(fmt.Sprintf("missing required dependency (%s:%d)", file, line))
		}
	}
}

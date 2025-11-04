/*
varz provides helpers to create expvar variables with package-qualified names.
It also imports expvar, so it will register it with http.DefaultServeMux.

varz, ironically, doesn't export /varz.  Maybe later.
*/
package varz

import (
	"expvar"
	"fmt"
	"runtime"
	"strings"
)

// callerPackage returns the package name of the caller of the
// function.  Use a loose heuristic to get that split apart.
// If the variable is declared in a var block, this will remove the
// "init" bit.
func callerPackage() string {
	// get package name of caller
	pc, _, _, ok := runtime.Caller(2)
	if !ok {
		return "varz.unknown"
	}
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return "varz.unknown"
	}

	n := fn.Name()
	dot := strings.LastIndex(n, ".")
	if dot != -1 {
		n = n[:dot]
	}

	return n
}

func NewInt(name string) *expvar.Int {
	return expvar.NewInt(fmt.Sprintf("%s.%s", callerPackage(), name))
}

func NewMap(name string) *expvar.Map {
	return expvar.NewMap(fmt.Sprintf("%s.%s", callerPackage(), name))
}

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
	"net/http"
	"strconv"

	"github.com/ts4z/irata/action"
	"github.com/ts4z/irata/assets"
	"github.com/ts4z/irata/defaults"
	"github.com/ts4z/irata/he"
	"github.com/ts4z/irata/model"
	"github.com/ts4z/irata/state"
	"github.com/ts4z/irata/textutil"
)

var templateFuncs template.FuncMap = template.FuncMap{
	"wrapLinesInNOBR": textutil.WrapLinesInNOBR,
	"joinNLNL":        textutil.JoinNLNL,
}

// func blitSpecialFile(w http.ResponseWriter, filename string) {
// 	rf, err := assets.Special.Open(filename)
// 	if err != nil {
// 		log.Fatalf("can't open special file: %v", err)
// 	}
// 	_, err = io.Copy(w, rf)
// 	if err != nil {
// 		log.Printf("error copying special static file to client: %v",
// 			err)
// 	}
// }

// TODO: make these configurable.

const listen = ":8888"
const databaseLocation = "postgresql:///irata"

// idPathValue extracts the "id" path variable from the request and parses it.
func idPathValue(w http.ResponseWriter, r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		log.Printf("400: can't parse id from url path: %v", err)
		http.Error(w, fmt.Sprintf("can't parse id from url path: %v", err), 400)
	}
	return id, nil
}

// irataApp prevents the proliferation of global variables.
type irataApp struct {
	templates *template.Template
	storage   state.Storage
	mutator   *action.Actor
	subFS     fs.FS
}

func (a *irataApp) fetchTournament(ctx context.Context, id int64) (*model.Tournament, error) {
	t, err := a.storage.FetchTournament(ctx, id)
	if err != nil {
		return nil, err
	}
	t.FillTransients()
	return t, nil
}

func (a *irataApp) cant(w http.ResponseWriter, code int, what string, err error) {
	log.Printf("%d: can't %s: %v", code, what, err)
	http.Error(w, fmt.Sprintf("can't %s: %v", what, err), code)
}

func (a *irataApp) installHandlers() {
	http.HandleFunc("/",
		func(w http.ResponseWriter, r *http.Request) {
			o, err := a.storage.FetchOverview(r.Context())
			if err != nil {
				a.cant(w, 500, "fetch overview", err)
				return
			}
			if err := a.templates.ExecuteTemplate(w, "slash.html.tmpl", o); err != nil {
				a.cant(w, 500, "render template", err)
				return
			}
		})

	// anything in fs is a file trivially shared
	http.Handle("/fs/", http.StripPrefix("/fs/", http.FileServer(http.FS(a.subFS))))

	http.HandleFunc("/t/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := idPathValue(w, r)
		if err != nil {
			return
		}

		t, err := a.fetchTournament(r.Context(), id) // todo: ID goes here
		if err != nil {
			a.cant(w, 404, "get tournament from database", err)
			return
		}
		if err := a.templates.ExecuteTemplate(w, "view.html.tmpl", t); err != nil {
			// don't use a.can't here, it would be a duplicate write to w
			log.Printf("500: can't render template: %v", err)
		}
	})

	http.HandleFunc("/edit/t/{id}", func(w http.ResponseWriter, r *http.Request) {
		id64, err := idPathValue(w, r)
		if err != nil {
			return
		}

		r.ParseForm()
		err = a.mutator.EditEvent(r.Context(), id64, r.Form)
		if err == nil {
			he.SendErrorToHTTPClient(w, "parsing form", err)
			return
		}

		t, err := a.fetchTournament(r.Context(), id64)
		if err != nil {
			he.SendErrorToHTTPClient(w, "fetching tournament", err)
			return
		}
		if err := a.templates.ExecuteTemplate(w, "edit.html.tmpl", t); err != nil {
			// don't use a.can't here, it would be a duplicate write to w
			log.Printf("500: can't render template: %v", err)
		}
	})

	http.HandleFunc("/api/footerPlugs/{id}", func(w http.ResponseWriter, r *http.Request) {
		_, err := idPathValue(w, r)
		if err != nil {
			return
		}
		bytes, err := json.Marshal(defaults.FooterPlugs())
		if err != nil {
			a.cant(w, 500, "marshal footerPlugs", err)
			return
		}
		writ, err := w.Write(bytes)
		if err != nil {
			log.Printf("error writing model to client: %v", err)
		} else if writ != len(bytes) {
			log.Println("short write to client")
		}
	})

	http.HandleFunc("/api/model/{id}", func(w http.ResponseWriter, r *http.Request) {
		id64, err := idPathValue(w, r)
		if err != nil {
			return
		}
		t, err := a.fetchTournament(r.Context(), id64)
		if err != nil {
			a.cant(w, 500, "get tourney from db", err)
			return
		}
		bytes, err := json.Marshal(t)
		if err != nil {
			a.cant(w, 500, "marshal model", err)
			return
		}
		writ, err := w.Write(bytes)
		if err != nil {
			log.Printf("error writing model to client: %v", err)
		} else if writ != len(bytes) {
			log.Println("short write to client")
		}
	})

	a.installKeyboardHandlers()
}

func (a *irataApp) installKeyboardHandlers() {
	var keyboardModifyEventHandlers = map[string]func(*model.Tournament) error{
		"PreviousLevel": func(t *model.Tournament) error { return t.PreviousLevel() },
		"SkipLevel":     func(t *model.Tournament) error { return t.AdvanceLevel() },
		"StopClock":     func(t *model.Tournament) error { return t.StopClock() },
		"StartClock":    func(t *model.Tournament) error { return t.StartClock() },
		"RemovePlayer":  func(t *model.Tournament) error { return t.RemovePlayer() },
		"AddPlayer":     func(t *model.Tournament) error { return t.AddPlayer() },
		"AddBuyIn":      func(t *model.Tournament) error { return t.AddBuyIn() },
		"RemoveBuyIn":   func(t *model.Tournament) error { return t.RemoveBuyIn() },
		"PlusMinute":    func(t *model.Tournament) error { return t.PlusMinute() },
		"MinusMinute":   func(t *model.Tournament) error { return t.MinusMinute() },
	}

	http.HandleFunc("/api/keyboard-control/{id}", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("keypress received")
		id64, err := idPathValue(w, r)
		if err != nil {
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			log.Printf("can't read response body: %v", err)
		}

		type KeyboardModifyEvent struct {
			Event string
		}

		var event KeyboardModifyEvent
		if err := json.Unmarshal(body, &event); err != nil {
			log.Printf("can't unmarshal event %s: %v", string(body), err)
		}

		if h, ok := keyboardModifyEventHandlers[event.Event]; !ok {
			http.Error(w, "unknown event", 404)
		} else {
			t, err := a.fetchTournament(r.Context(), id64)
			if err != nil {
				http.Error(w, fmt.Sprintf("tournament not found %v", err), 404)
			}

			if err := h(t); err != nil {
				a.cant(w, 500, "applying keyboard event", err)
			}

			if err := a.storage.SaveTournament(r.Context(), t); err != nil {
				a.cant(w, 500, "save tournament after keypress", err)
			}
		}
	})
}

func (a *irataApp) loadTemplates() {
	var err error
	if a.templates, err = template.New("root").Funcs(templateFuncs).ParseFS(assets.Templates, "templates/*[^~]"); err != nil {
		log.Fatalf("error loading embedded templates: %v", err)
	}
	for _, tmpl := range a.templates.Templates() {
		// tmpl.Funcs(templateFuncs)
		log.Printf("loaded template %q", tmpl.Name())
	}
}

func (a *irataApp) serve() error {
	return http.ListenAndServe(":8888", nil)
}

func main() {
	subFS, err := fs.Sub(assets.FS, "fs")
	if err != nil {
		log.Fatalf("fs.Sub: %v", err)
	}

	storage, err := state.NewDBStorage(context.Background(), databaseLocation)
	if err != nil {
		log.Fatalf("can't configure database: %v", err)
	}

	mutator := action.New(storage)

	app := &irataApp{storage: storage, mutator: mutator, subFS: subFS}
	app.loadTemplates()
	app.installHandlers()
	if err := app.serve(); err != nil {
		log.Fatalf("can't serve: %v", err)
	}
}

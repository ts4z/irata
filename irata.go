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
	"time"

	"github.com/ts4z/irata/action"
	"github.com/ts4z/irata/assets"
	"github.com/ts4z/irata/defaults"
	"github.com/ts4z/irata/he"
	"github.com/ts4z/irata/model"
	"github.com/ts4z/irata/password"
	"github.com/ts4z/irata/permission"
	"github.com/ts4z/irata/state"
	"github.com/ts4z/irata/textutil"
	"github.com/ts4z/irata/ts"
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

const listenAddress = ":8888"
const dbURL = "postgresql:///irata"

// idPathValue extracts the "id" path variable from the request and parses it.
func idPathValue(w http.ResponseWriter, r *http.Request) (int64, error) {
	id, err := idPathValueFromRequest(r)
	if err != nil {
		he.SendErrorToHTTPClient(w, "parsing URL", err)
	}
	return id, nil
}

func idPathValueFromRequest(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return -1, he.HTTPCodedErrorf(400, "can't parse id from url path: %v", err)
	}
	return id, nil
}

// irataApp prevents the proliferation of global variables.
type irataApp struct {
	templates   *template.Template
	storage     state.AppStorage
	userStorage state.UserStorage
	mutator     *action.Actor
	subFS       fs.FS
	bakery      *permission.Bakery
	clock       *ts.Clock
}

func (app *irataApp) fetchTournament(ctx context.Context, id int64) (*model.Tournament, error) {
	t, err := app.storage.FetchTournament(ctx, id)
	if err != nil {
		return nil, err
	}
	t.FillTransients(app.clock)
	return t, nil
}

func (app *irataApp) fetchUserFromCookie(ctx context.Context, r *http.Request) (*model.UserIdentity, error) {
	cookieData, err := app.bakery.ReadCookie(r)
	if err != nil {
		return nil, err
	}

	identity, err := app.userStorage.FetchUserByUserID(ctx, cookieData.EffectiveUserID)
	if err != nil {
		log.Printf("can't fetch user %+v: %v", cookieData.EffectiveUserID, err)
	}
	return identity, nil
}

func (app *irataApp) saveUserInRequestContext(r *http.Request) (context.Context, *model.UserIdentity) {
	ctx := r.Context()
	identity, err := app.fetchUserFromCookie(ctx, r)
	if err != nil {
		return ctx, nil
	}
	return permission.UserIdentityInContext(ctx, identity), identity
}

func (app *irataApp) installHandlers() {
	http.HandleFunc("/",
		func(w http.ResponseWriter, r *http.Request) {
			ctx, _ := app.saveUserInRequestContext(r)

			// TODO: pagination
			o, err := app.storage.FetchOverview(ctx, 0, 100)
			if err != nil {
				he.SendErrorToHTTPClient(w, "fetch overview", err)
				return
			}
			type Inputs struct {
				IsAdmin  bool
				Overview *model.Overview
			}
			inputs := &Inputs{IsAdmin: permission.IsAdmin(ctx), Overview: o}
			if err := app.templates.ExecuteTemplate(w, "slash.html.tmpl", inputs); err != nil {
				log.Printf("can't render template: %v", err)
				return
			}
		})

	http.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFileFS(w, r, app.subFS, "favicon.ico")
	})

	http.HandleFunc("/logout", func(w http.ResponseWriter, r *http.Request) {
		app.bakery.ClearCookie(w)
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})

	// anything in fs is a file trivially shared
	http.Handle("/fs/", http.StripPrefix("/fs/", http.FileServer(http.FS(app.subFS))))

	renderTournament := func(w http.ResponseWriter, r *http.Request) {
		ctx, _ := app.saveUserInRequestContext(r)

		id, err := idPathValue(w, r)
		if err != nil {
			return
		}

		t, err := app.fetchTournament(ctx, id)
		if err != nil {
			he.SendErrorToHTTPClient(w, "get tournament from database", err)
			return
		}
		if err := app.templates.ExecuteTemplate(w, "view.html.tmpl", t); err != nil {
			log.Printf("500: can't render template: %v", err)
		}
	}
	http.HandleFunc("/t/{id}", renderTournament)

	http.HandleFunc("/t/{id}/delete", func(w http.ResponseWriter, r *http.Request) {
		id64, err := idPathValue(w, r)
		if err != nil {
			return
		}

		if err := app.storage.DeleteTournament(r.Context(), id64); err != nil {
			he.SendErrorToHTTPClient(w, "deleting tournament", err)
		} else {
			http.Redirect(w, r, "/", http.StatusPermanentRedirect)
		}
	})

	http.HandleFunc("/t/{id}/edit", func(w http.ResponseWriter, r *http.Request) {
		ctx, _ := app.saveUserInRequestContext(r)

		id64, err := idPathValue(w, r)
		if err != nil {
			return
		}

		r.ParseForm()
		if len(r.Form) != 0 {
			err = app.mutator.EditEvent(ctx, id64, r.Form)
			if err != nil {
				he.SendErrorToHTTPClient(w, "parsing form", err)
				return
			}
			http.Redirect(w, r, fmt.Sprintf("/t/%d", id64), http.StatusSeeOther)
			return
		}

		t, err := app.fetchTournament(ctx, id64)
		if err != nil {
			he.SendErrorToHTTPClient(w, "fetching tournament", err)
			return
		}

		args := &struct {
			Tournament *model.Tournament
			IsAdmin    bool
		}{
			Tournament: t,
			IsAdmin:    permission.IsAdmin(ctx),
		}

		if err := app.templates.ExecuteTemplate(w, "edit.html.tmpl", args); err != nil {
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
			he.SendErrorToHTTPClient(w, "marshalling plugs", he.New(500, err))
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
		t, err := app.fetchTournament(r.Context(), id64)
		if err != nil {
			he.SendErrorToHTTPClient(w, "get tourney from db", err)
			return
		}
		bytes, err := json.Marshal(t)
		if err != nil {
			he.SendErrorToHTTPClient(w, "marshal model", err)
			return
		}
		writ, err := w.Write(bytes)
		if err != nil {
			log.Printf("error writing model to client: %v", err)
		} else if writ != len(bytes) {
			log.Println("short write to client")
		}
	})

	http.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		switch r.Method {
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		case http.MethodGet:
			flash := r.URL.Query().Get("error")
			data := struct {
				Flash string
			}{Flash: flash}
			if err := app.templates.ExecuteTemplate(w, "login.html.tmpl", data); err != nil {
				log.Printf("can't render login template: %v", err)
			}
			return
		case http.MethodPost:
			nope := func() {
				http.Redirect(w, r, "/login?error=internal+error", http.StatusSeeOther)
			}

			if err := r.ParseForm(); err != nil {
				he.SendErrorToHTTPClient(w, "parse login form", err)
				return
			}
			nick := r.FormValue("username")
			pw := r.FormValue("password")
			if nick == "" || pw == "" {
				http.Redirect(w, r, "/login?error=username+and+password+required", http.StatusSeeOther)
				return
			}

			row, err := app.userStorage.FetchUserRow(ctx, nick)
			if err != nil {
				nope()
				return
			}
			checker, err := password.NewChecker(row)
			if err != nil {
				nope()
				return
			}
			identity, err := checker.Validate(pw)
			if err != nil {
				nope()
				return
			}
			err = app.bakery.BakeCookie(w, &model.AuthCookieData{
				RealUserID:      identity.ID,
				EffectiveUserID: identity.ID,
			})
			if err != nil {
				http.Redirect(w, r, "/login?error=internal+error+baking+cookie", http.StatusSeeOther)
			}

			http.Redirect(w, r, "/", http.StatusSeeOther)
		}
	})

	app.installKeyboardHandlers()
}

func (app *irataApp) handleKeypress(r *http.Request) error {
	var keyboardModifyEventHandlers = map[string]func(*model.Tournament) error{
		"PreviousLevel": func(t *model.Tournament) error { return t.PreviousLevel(app.clock) },
		"SkipLevel":     func(t *model.Tournament) error { return t.AdvanceLevel(app.clock) },
		"StopClock":     func(t *model.Tournament) error { return t.StopClock(app.clock) },
		"StartClock":    func(t *model.Tournament) error { return t.StartClock(app.clock) },
		"RemovePlayer":  func(t *model.Tournament) error { return t.RemovePlayer(app.clock) },
		"AddPlayer":     func(t *model.Tournament) error { return t.AddPlayer(app.clock) },
		"AddBuyIn":      func(t *model.Tournament) error { return t.AddBuyIn(app.clock) },
		"RemoveBuyIn":   func(t *model.Tournament) error { return t.RemoveBuyIn(app.clock) },
		"PlusMinute":    func(t *model.Tournament) error { return t.PlusMinute(app.clock) },
		"MinusMinute":   func(t *model.Tournament) error { return t.MinusMinute(app.clock) },
	}

	ctx, _ := app.saveUserInRequestContext(r)

	log.Printf("keypress received")
	id64, err := idPathValueFromRequest(r)
	if err != nil {
		return err
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
		return he.HTTPCodedErrorf(404, "unknown keyboard event")
	} else {
		t, err := app.fetchTournament(r.Context(), id64)
		if err != nil {
			return he.HTTPCodedErrorf(404, "tournament not found: %w", err)
		}

		if err := h(t); err != nil {
			return he.HTTPCodedErrorf(500, "while applying keyboard event: %w", err)
		}

		if err := app.storage.SaveTournament(ctx, t); err != nil {
			return he.HTTPCodedErrorf(500, "save tournament after keypress: %w", err)
		}
	}
	return nil
}

func (app *irataApp) installKeyboardHandlers() {
	http.HandleFunc("/api/keyboard-control/{id}", func(w http.ResponseWriter, r *http.Request) {
		err := app.handleKeypress(r)
		if err != nil {
			he.SendErrorToHTTPClient(w, "handling keypress", err)
		}
	})
}

func (app *irataApp) loadTemplates() {
	var err error
	if app.templates, err = template.New("root").Funcs(templateFuncs).ParseFS(assets.Templates, "templates/*[^~]"); err != nil {
		log.Fatalf("error loading embedded templates: %v", err)
	}
	for _, tmpl := range app.templates.Templates() {
		// tmpl.Funcs(templateFuncs)
		log.Printf("loaded template %q", tmpl.Name())
	}
}

func (app *irataApp) serve() error {
	return http.ListenAndServe(listenAddress, nil)
}

func readSiteConfig(ctx context.Context, s state.SiteStorage) (*model.SiteConfig, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return s.FetchSiteConfig(ctx)
}

func main() {
	ctx := context.Background()
	clock := ts.NewRealClock()
	subFS, err := fs.Sub(assets.FS, "fs")
	if err != nil {
		log.Fatalf("fs.Sub: %v", err)
	}

	unprotectedStorage, err := state.NewDBStorage(context.Background(), dbURL)
	if err != nil {
		log.Fatalf("can't configure database: %v", err)
	}

	siteConfig, err := readSiteConfig(ctx, unprotectedStorage)
	if err != nil {
		log.Fatalf("can't fetch site config: %v", err)
	}

	bakery, err := permission.New(clock, siteConfig)
	if err != nil {
		log.Fatalf("can't create bakery: %v", err)
	}

	storage := &permission.StorageDecorator{Storage: unprotectedStorage}

	mutator := action.New(storage)

	app := &irataApp{storage: storage, userStorage: unprotectedStorage,
		mutator: mutator, subFS: subFS, bakery: bakery, clock: clock}
	app.loadTemplates()
	app.installHandlers()
	if err := app.serve(); err != nil {
		log.Fatalf("can't serve: %v", err)
	}
}

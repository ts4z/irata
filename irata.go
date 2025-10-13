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
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ts4z/irata/action"
	"github.com/ts4z/irata/assets"
	"github.com/ts4z/irata/he"
	"github.com/ts4z/irata/model"
	"github.com/ts4z/irata/password"
	"github.com/ts4z/irata/permission"
	"github.com/ts4z/irata/state"
	"github.com/ts4z/irata/textutil"
	"github.com/ts4z/irata/ts"
	"github.com/ts4z/irata/urlpath"
)

var templateFuncs template.FuncMap = template.FuncMap{
	"wrapLinesInNOBR": textutil.WrapLinesInNOBR,
	"joinNLNL":        textutil.JoinNLNL,
}

// TODO: make these configurable.

const listenAddress = ":8888"
const dbURL = "postgresql:///irata"

func idPathValue(w http.ResponseWriter, r *http.Request) (int64, error) {
	return urlpath.IDPathValue(w, r)
}

type nower interface {
	Now() time.Time
}

// irataApp prevents the proliferation of global variables.
type irataApp struct {
	templates   *template.Template
	storage     state.AppStorage
	siteStorage state.SiteStorage
	userStorage state.UserStorage
	mutator     *action.Actor
	subFS       fs.FS
	bakery      *permission.Bakery
	clock       nower

	keypressHandlers map[string]func(*model.Tournament) error

	mux     *http.ServeMux
	handler http.Handler
}

func (app *irataApp) fetchTournament(ctx context.Context, id int64) (*model.Tournament, error) {
	t, err := app.storage.FetchTournament(ctx, id)
	if err != nil {
		return nil, err
	}
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

var RegexpLFLF = regexp.MustCompile(`\n\n+`)

func parseFooterPlugsBox(plugsRaw string) []string {
	plugs := []string{}
	plugsRaw = strings.ReplaceAll(plugsRaw, "\r", "")
	for _, plug := range RegexpLFLF.Split(plugsRaw, -1) {
		plug = strings.TrimSpace(plug)
		if plug != "" {
			plugs = append(plugs, plug)
		}
	}
	return plugs
}

type CodeWatcher struct {
	code *int
	w    http.ResponseWriter
}

func (cw *CodeWatcher) Header() http.Header {
	return cw.w.Header()
}

func (cw *CodeWatcher) Write(b []byte) (int, error) {
	return cw.w.Write(b)
}

func (cw *CodeWatcher) WriteHeader(statusCode int) {
	cw.code = &statusCode
	cw.w.WriteHeader(statusCode)
}

func (cw *CodeWatcher) Code() int {
	if cw.code != nil {
		return *cw.code
	} else {
		return 200
	}
}

var _ http.ResponseWriter = &CodeWatcher{}

type RequestLogger struct {
	next http.Handler
}

func (rl *RequestLogger) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ww := &CodeWatcher{w: w}
	rl.next.ServeHTTP(ww, r)
	code := ww.Code()
	log.Printf("[access] %d %v", code, r.URL.Path)
}

type CookieParser struct {
	app  *irataApp
	next http.Handler
}

func (cp *CookieParser) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	identity, err := cp.app.fetchUserFromCookie(ctx, r)
	if err != nil {
		log.Printf("can't fetch user data from cookie: %v", err)
	} else {
		ctx = permission.UserIdentityInContext(ctx, identity)
		r = r.WithContext(ctx)
	}

	cp.next.ServeHTTP(w, r)
}

func (app *irataApp) requiringAdminHandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	app.mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if !permission.IsAdmin(ctx) {
			he.SendErrorToHTTPClient(w, "authorize", he.HTTPCodedErrorf(http.StatusUnauthorized, "permission denied"))
			return
		}
		handler(w, r)
	})
}

func (app *irataApp) installHandlers() {
	// Handler for /manage/structure
	app.requiringAdminHandleFunc("/manage/structure", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		// Fetch all structures
		slugs, err := app.storage.FetchStructureSlugs(ctx, 0, 100)
		if err != nil {
			he.SendErrorToHTTPClient(w, "fetch structure slugs", err)
			return
		}
		structures := []*model.Structure{}
		for _, slug := range slugs {
			st, err := app.storage.FetchStructure(ctx, slug.ID)
			if err == nil {
				structures = append(structures, st)
			}
		}
		data := struct {
			Structures []*model.Structure
		}{Structures: structures}
		if err := app.templates.ExecuteTemplate(w, "manage-structure.html.tmpl", data); err != nil {
			log.Printf("can't render manage-structure template: %v", err)
		}
	})

	app.requiringAdminHandleFunc("/create/t", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		var flash string
		// Fetch available structures and footer plug sets
		structures, err := app.storage.FetchStructureSlugs(ctx, 0, 100)
		if err != nil {
			he.SendErrorToHTTPClient(w, "fetch structure slugs", err)
			return
		}
		footers, err := app.storage.ListFooterPlugSets(ctx)
		if err != nil {
			he.SendErrorToHTTPClient(w, "fetch footer plug sets", err)
			return
		}
		if r.Method == http.MethodPost {
			if err := r.ParseForm(); err != nil {
				flash = "Error parsing form"
			} else {
				eventName := r.FormValue("EventName")
				handle := r.FormValue("Handle")
				description := r.FormValue("Description")
				prizePool := r.FormValue("PrizePool")
				structureID, _ := strconv.ParseInt(r.FormValue("StructureID"), 10, 64)
				footerPlugsID, _ := strconv.ParseInt(r.FormValue("FooterPlugsID"), 10, 64)
				if eventName == "" || handle == "" || structureID == 0 || footerPlugsID == 0 {
					flash = "All required fields must be filled"
				} else {
					// Fetch structure and denormalize into tournament
					structure, err := app.storage.FetchStructure(ctx, structureID)
					if err != nil {
						log.Printf("error fetching structure %d: %v", structureID, err)
						flash = "Error fetching structure"
					} else {
						t := &model.Tournament{
							EventName:     eventName,
							Handle:        handle,
							Description:   description,
							FooterPlugsID: footerPlugsID,
							Structure:     &structure.StructureData,
							State:         &model.State{PrizePool: prizePool},
						}
						id, err := app.storage.CreateTournament(ctx, t)
						if err != nil {
							log.Printf("error creating tournament: %v", err)
							flash = "Error creating tournament (is the handle not unique?)"
						} else {
							http.Redirect(w, r, fmt.Sprintf("/t/%d", id), http.StatusSeeOther)
							return
						}
					}
				}
			}
		}
		data := struct {
			Structures []*model.StructureSlug
			FooterSets []*model.FooterPlugs
			Flash      string
		}{Structures: structures, FooterSets: footers, Flash: flash}
		if err := app.templates.ExecuteTemplate(w, "create-tournament.html.tmpl", data); err != nil {
			log.Printf("can't render create-tournament template: %v", err)
		}
	})

	app.requiringAdminHandleFunc("/manage/structure/{id}/edit", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		id, err := idPathValue(w, r)
		if err != nil {
			return
		}
		var flash string
		if r.Method == http.MethodPost {
			if err := r.ParseForm(); err != nil {
				flash = "Error parsing form"
			} else {
				name := r.FormValue("Name")
				levels := []*model.Level{}
				for i := 0; ; i++ {
					durStr := r.FormValue(fmt.Sprintf("Level%dDuration", i))
					desc := r.FormValue(fmt.Sprintf("Level%dDescription", i))
					isBreak := r.FormValue(fmt.Sprintf("Level%dIsBreak", i)) == "on"
					if durStr == "" && desc == "" && !isBreak && i > 0 {
						break
					}
					if durStr == "" && desc == "" {
						continue
					}
					dur, err := strconv.Atoi(durStr)
					if err != nil || dur <= 0 || desc == "" {
						flash = "All fields required for each level"
						continue
					}
					levels = append(levels, &model.Level{
						DurationMinutes: dur,
						Description:     desc,
						IsBreak:         isBreak,
						Banner:          desc,
					})
				}
				if name == "" || len(levels) == 0 {
					flash = "Structure name and at least one level required"
				} else {
					st, err := app.storage.FetchStructure(ctx, id)
					if err != nil {
						flash = "Error fetching structure"
					} else {
						st.Name = name
						st.Levels = levels
						err := app.storage.SaveStructure(ctx, st)
						if err != nil {
							flash = "Error saving structure"
						} else {
							http.Redirect(w, r, "/manage/structure", http.StatusSeeOther)
							return
						}
					}
				}
			}
		}
		st, err := app.storage.FetchStructure(ctx, id)
		if err != nil {
			he.SendErrorToHTTPClient(w, "fetch structure", err)
			return
		}
		data := struct {
			Structure *model.Structure
			Flash     string
		}{Structure: st, Flash: flash}
		if err := app.templates.ExecuteTemplate(w, "edit-structure.html.tmpl", data); err != nil {
			log.Printf("can't render edit-structure template: %v", err)
		}
	})

	app.requiringAdminHandleFunc("/manage/footer-set/{id}/edit", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		id, err := idPathValue(w, r)
		if err != nil {
			return
		}

		var flash string
		if r.Method == http.MethodPost {
			if err := r.ParseForm(); err != nil {
				flash = "Error parsing form"
			} else {
				name := r.FormValue("Name")
				plugs := []string{}
				for i := 0; ; i++ {
					plug := r.FormValue(fmt.Sprintf("Plug%d", i))
					if plug == "" && i >= len(r.Form)-1 {
						break
					}
					plug = strings.TrimSpace(plug)
					if plug != "" {
						plugs = append(plugs, plug)
					}
				}
				if name == "" {
					flash = "Set name required"
				} else {
					err := app.storage.UpdateFooterPlugSet(ctx, id, name, plugs)
					if err != nil {
						flash = "Error saving footer plug set"
					} else {
						http.Redirect(w, r, fmt.Sprintf("/manage/footer-set/%d/edit", id), http.StatusSeeOther)
						return
					}
				}
			}
		}
		fp, err := app.storage.FetchPlugs(ctx, id)
		if err != nil {
			he.SendErrorToHTTPClient(w, "fetch footer plug set", err)
			return
		}
		data := struct {
			FooterSet *model.FooterPlugs
			Flash     string
		}{FooterSet: fp, Flash: flash}
		if err := app.templates.ExecuteTemplate(w, "edit-footer-set.html.tmpl", data); err != nil {
			log.Printf("can't render edit-footer-set template: %v", err)
		}
	})

	app.requiringAdminHandleFunc("/create/footer-set", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		var flash string
		if r.Method == http.MethodPost {
			if err := r.ParseForm(); err != nil {
				flash = "Error parsing form"
			} else {
				name := r.FormValue("Name")
				plugsRaw := r.FormValue("Plugs")
				if name == "" || plugsRaw == "" {
					flash = "All fields required"
				} else {
					plugs := parseFooterPlugsBox(plugsRaw)
					id, err := app.storage.CreateFooterPlugSet(ctx, name, plugs)
					if err != nil {
						log.Printf("error creating footer plug set: %v", err)
						flash = "Error saving footer plug set"
					} else {
						http.Redirect(w, r, fmt.Sprintf("/manage/footer-set/%d", id), http.StatusSeeOther)
						return
					}
				}
			}
		}
		data := struct{ Flash string }{Flash: flash}
		if err := app.templates.ExecuteTemplate(w, "create-footer-set.html.tmpl", data); err != nil {
			log.Printf("can't render create-footer-set template: %v", err)
		}
	})

	app.requiringAdminHandleFunc("/manage/footer-set/{id}/delete", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		id, err := idPathValue(w, r)
		if err != nil {
			return
		}

		if r.Method != http.MethodPost {
			// Render a simple confirmation page
			fmt.Fprintf(w, "<html><body><h2>Delete Footer Plug Set %d?</h2><form method='POST'><button type='submit'>Delete</button> <a href='/manage/footer-sets'>Cancel</a></form></body></html>", id)
			return
		}
		err = app.storage.DeleteFooterPlugSet(ctx, id)
		if err != nil {
			he.SendErrorToHTTPClient(w, "delete footer plug set", err)
			return
		}
		http.Redirect(w, r, "/manage/footers", http.StatusSeeOther)
	})

	app.requiringAdminHandleFunc("/manage/footer-set/", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		sets, err := app.storage.ListFooterPlugSets(ctx)
		if err != nil {
			he.SendErrorToHTTPClient(w, "fetch footer plug sets", err)
			return
		}

		data := struct {
			FooterSets []*model.FooterPlugs
		}{FooterSets: sets}
		if err := app.templates.ExecuteTemplate(w, "manage-footer-sets.html.tmpl", data); err != nil {
			log.Printf("can't render manage-footer-sets template: %v", err)
		}
	})

	app.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		sc, err := app.siteStorage.FetchSiteConfig(ctx)
		if err != nil {
			he.SendErrorToHTTPClient(w, "fetch site config", err)
			return
		}

		// TODO: pagination
		o, err := app.storage.FetchOverview(ctx, 0, 100)
		if err != nil {
			he.SendErrorToHTTPClient(w, "fetch overview", err)
			return
		}
		type Inputs struct {
			IsAdmin    bool
			Overview   *model.Overview
			SiteConfig *model.SiteConfig
		}
		inputs := &Inputs{IsAdmin: permission.IsAdmin(ctx), Overview: o, SiteConfig: sc}
		if err := app.templates.ExecuteTemplate(w, "slash.html.tmpl", inputs); err != nil {
			log.Printf("can't render template: %v", err)
			return
		}
	})

	app.mux.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFileFS(w, r, app.subFS, "favicon.ico")
	})

	app.mux.HandleFunc("/logout", func(w http.ResponseWriter, r *http.Request) {
		app.bakery.ClearCookie(w)
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})

	// anything in fs is a file trivially shared
	app.mux.Handle("/fs/", http.StripPrefix("/fs/", http.FileServer(http.FS(app.subFS))))

	renderTournament := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		id, err := idPathValue(w, r)
		if err != nil {
			return
		}

		t, err := app.fetchTournament(ctx, id)
		if err != nil {
			he.SendErrorToHTTPClient(w, "get tournament from database", err)
			return
		}
		args := struct {
			Tournament              *model.Tournament
			InstallKeyboardHandlers bool
		}{
			Tournament:              t,
			InstallKeyboardHandlers: permission.CheckWriteAccessToTournamentID(ctx, id) == nil,
		}
		log.Printf("render with args: %+v", args)
		if err := app.templates.ExecuteTemplate(w, "view.html.tmpl", args); err != nil {
			log.Printf("500: can't render template: %v", err)
		}
	}
	app.mux.HandleFunc("/t/{id}", renderTournament)

	app.requiringAdminHandleFunc("/t/{id}/delete", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		id64, err := idPathValue(w, r)
		if err != nil {
			return
		}

		if err := app.storage.DeleteTournament(ctx, id64); err != nil {
			he.SendErrorToHTTPClient(w, "delete tournament", err)
		} else {
			http.Redirect(w, r, "/", http.StatusPermanentRedirect)
		}
	})

	app.requiringAdminHandleFunc("/t/{id}/edit", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

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

	app.mux.HandleFunc("/api/footerPlugs/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := idPathValue(w, r)
		if err != nil {
			return
		}

		fp, err := app.storage.FetchPlugs(r.Context(), id)
		if err != nil {
			he.SendErrorToHTTPClient(w, "get plugs from db", err)
			return
		}

		for i, s := range fp.TextPlugs {
			fp.TextPlugs[i] = textutil.WrapLinesInNOBR(s)
		}

		bytes, err := json.Marshal(fp)
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

	app.mux.HandleFunc("/api/model/{id}", func(w http.ResponseWriter, r *http.Request) {
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

	app.mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
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
			checker, err := password.NewChecker(app.clock, row)
			if err != nil {
				nope()
				return
			}
			identity, err := checker.Validate(pw)
			if err != nil {
				http.Redirect(w, r, "/login?error=invalid+user+or+password", http.StatusSeeOther)
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

	// Handler for /api/tournament-listen
	app.mux.HandleFunc("/api/tournament-listen", func(w http.ResponseWriter, r *http.Request) {
		type reqBody struct {
			TournamentID int64 `json:"tournament_id"`
			Version      int64 `json:"version"`
		}
		var req reqBody
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Printf("can't decode body: %v", err)
			he.SendErrorToHTTPClient(w, "/api/tournament-listen", he.HTTPCodedErrorf(400, "decoding json: %w", err))
			return
		}
		errCh := make(chan error, 1)
		tournamentCh := make(chan *model.Tournament, 1)
		timeoutCh := time.After(time.Hour)
		go app.storage.ListenTournamentVersion(r.Context(), req.TournamentID, req.Version, errCh, tournamentCh)
		select {
		case err := <-errCh:
			he.SendErrorToHTTPClient(w, "listening for tournament version change", err)
			return
		case tm := <-tournamentCh:
			bytes, err := json.Marshal(tm)
			if err != nil {
				he.SendErrorToHTTPClient(w, "marshal model", err)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write(bytes)
			return
		case <-timeoutCh:
			he.SendErrorToHTTPClient(w, "waiting for tournament update",
				he.HTTPCodedErrorf(http.StatusGatewayTimeout, "timeout"))
			return
		}
	})

	app.installKeyboardHandlers()

	// Handler for /manage/site
	app.requiringAdminHandleFunc("/manage/site", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		var flash string
		// Fetch config
		config, err := readSiteConfig(ctx, app.siteStorage)
		if err != nil {
			he.SendErrorToHTTPClient(w, "fetch site config", err)
			return
		}

		if r.Method == http.MethodPost {
			if err := r.ParseForm(); err != nil {
				flash = "Error parsing form"
			} else {
				name := r.FormValue("Name")
				site := r.FormValue("Site")
				theme := r.FormValue("Theme")
				if name == "" || site == "" || theme == "" {
					flash = "All fields required"
				} else {
					// Update config
					config.Name = name
					config.Site = site
					config.Theme = theme
					err := app.siteStorage.SaveSiteConfig(ctx, config)
					if err != nil {
						flash = "Error saving config"
					} else {
						flash = "Saved!"
					}
				}
			}
		}

		data := struct {
			Config *model.SiteConfig
			Flash  string
		}{Config: config, Flash: flash}
		if err := app.templates.ExecuteTemplate(w, "manage-site.html.tmpl", data); err != nil {
			log.Printf("can't render manage-site template: %v", err)
		}
	})
}

func makeKeyboardHandlers(clock ts.Clock) map[string]func(*model.Tournament) error {
	// todo: it is bogus that these require a clock.  it would make more sense if these methods
	// were moved outside the model, since they are not just data, but actual actions.
	return map[string]func(*model.Tournament) error{
		"PreviousLevel": func(t *model.Tournament) error { return t.PreviousLevel(clock) },
		"SkipLevel":     func(t *model.Tournament) error { return t.AdvanceLevel(clock) },
		"StopClock":     func(t *model.Tournament) error { return t.StopClock(clock) },
		"StartClock":    func(t *model.Tournament) error { return t.StartClock(clock) },
		"RemovePlayer":  func(t *model.Tournament) error { return t.RemovePlayer(clock) },
		"AddPlayer":     func(t *model.Tournament) error { return t.AddPlayer(clock) },
		"AddBuyIn":      func(t *model.Tournament) error { return t.AddBuyIn(clock) },
		"RemoveBuyIn":   func(t *model.Tournament) error { return t.RemoveBuyIn(clock) },
		"PlusMinute":    func(t *model.Tournament) error { return t.PlusMinute(clock) },
		"MinusMinute":   func(t *model.Tournament) error { return t.MinusMinute(clock) },
	}
}

func (app *irataApp) handleKeypress(r *http.Request) error {
	ctx := r.Context()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("can't read response body: %v", err)
	}

	type KeyboardModifyEvent struct {
		TournamentID int64
		Event        string
	}

	var event KeyboardModifyEvent
	if err := json.Unmarshal(body, &event); err != nil {
		log.Printf("can't unmarshal event %s: %v", string(body), err)
	}

	// Redundant check (storage checks too) to marginally improve logs + error.
	if permission.CheckWriteAccessToTournamentID(ctx, event.TournamentID) != nil {
		return he.HTTPCodedErrorf(http.StatusUnauthorized, "permission denied")
	}

	if h, ok := app.keypressHandlers[event.Event]; !ok {
		return he.HTTPCodedErrorf(404, "unknown keyboard event")
	} else {
		t, err := app.fetchTournament(r.Context(), event.TournamentID)
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
	app.requiringAdminHandleFunc("/api/keyboard-control", func(w http.ResponseWriter, r *http.Request) {
		err := app.handleKeypress(r)
		if err != nil {
			he.SendErrorToHTTPClient(w, "handleKeypress", err)
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
	wg := sync.WaitGroup{}

	type result struct {
		name string
		err  error
	}

	ch := make(chan *result)

	wg.Add(1)
	go func() {
		ch <- &result{"http", http.ListenAndServe(listenAddress, app.handler)}
		wg.Done()
	}()

	// wg.Add(1)
	// go func() {
	// 	ch <- &result{"http", http.ListenAndServe(listenAddress, app.handler)}
	// 	wg.Done()
	// }()

	go func() {
		wg.Wait()
		close(ch)
	}()

	errors := []error{}
	for res := range ch {
		if res.err != nil {
			log.Printf("server %s exited: %v", res.name, res.err)
			errors = append(errors, res.err)
		}
	}

	return fmt.Errorf("servers exited: %v", errors)
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

	unprotectedStorage, err := state.NewDBStorage(context.Background(), dbURL, clock)
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

	storage := &permission.StoragePermissionFacade{Storage: unprotectedStorage}

	mutator := action.New(storage)

	app := &irataApp{storage: storage, siteStorage: unprotectedStorage, userStorage: unprotectedStorage,
		mutator: mutator, subFS: subFS, bakery: bakery, clock: clock}
	app.mux = http.NewServeMux()
	app.handler = &RequestLogger{next: &CookieParser{app: app, next: app.mux}}
	app.keypressHandlers = makeKeyboardHandlers(clock)

	app.loadTemplates()
	app.installHandlers()
	if err := app.serve(); err != nil {
		log.Fatalf("can't serve: %v", err)
	}
}

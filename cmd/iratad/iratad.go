package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
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

	"github.com/jonboulle/clockwork"
	"github.com/spf13/viper"

	"github.com/ts4z/irata/action"
	"github.com/ts4z/irata/app/handlers"
	"github.com/ts4z/irata/assets"
	"github.com/ts4z/irata/config"
	"github.com/ts4z/irata/he"
	"github.com/ts4z/irata/middleware"
	"github.com/ts4z/irata/middleware/c2ctx"
	"github.com/ts4z/irata/middleware/labrea"
	"github.com/ts4z/irata/model"
	"github.com/ts4z/irata/password"
	"github.com/ts4z/irata/paytable"
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

func idPathValue(w http.ResponseWriter, r *http.Request) (int64, error) {
	return urlpath.IDPathValue(w, r)
}

type nower interface {
	Now() time.Time
}

type modifiers struct {
	Shift bool
}

type editTournamentArgs struct {
	Flash      string
	Tournament *model.Tournament
	Structures []*model.StructureSlug
	FooterSets []*model.FooterPlugs
	Paytables  []*paytable.PaytableSlug
	IsAdmin    bool
	IsNew      bool
	SiteConfig *model.SiteConfig
}

// irataApp prevents the proliferation of global variables.
type irataApp struct {
	listenAddress string

	templates       *template.Template
	appStorage      state.AppStorage
	siteStorage     state.SiteStorage
	userStorage     state.UserStorage
	paytableStorage state.PaytableStorage
	mutator         *action.Actor
	subFS           fs.FS
	bakery          *permission.Bakery
	clock           nower
	modelDeps       *model.Deps

	keypressHandlers map[string]func(*model.Tournament, *modifiers) error

	mux     *http.ServeMux
	handler http.Handler
}

func (app *irataApp) fetchTournament(ctx context.Context, id int64) (*model.Tournament, error) {
	t, err := app.appStorage.FetchTournament(ctx, id)
	if err != nil {
		return nil, err
	}
	return t, nil
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

func (app *irataApp) handleFunc(pattern string, handler func(context.Context, http.ResponseWriter, *http.Request)) {
	app.mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		handler(ctx, w, r)
	})
}

func (app *irataApp) handleFuncTakingID(pattern string, handler func(context.Context, int64, http.ResponseWriter, *http.Request)) {
	app.handleFunc(pattern, func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		id, err := idPathValue(w, r)
		if err != nil {
			he.SendErrorToHTTPClient(w, "parse url", err)
		}
		handler(ctx, id, w, r)
	})
}

func (app *irataApp) requiringAdminHandleFunc(pattern string, handler func(context.Context, http.ResponseWriter, *http.Request)) {
	app.handleFunc(pattern, func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		if !permission.IsAdmin(ctx) {
			he.SendErrorToHTTPClient(w, "authorize", he.HTTPCodedErrorf(http.StatusUnauthorized, "permission denied"))
			return
		}
		handler(ctx, w, r)
	})
}

func (app *irataApp) requiringAdminTakingIDHandleFunc(pattern string, handler func(context.Context, int64, http.ResponseWriter, *http.Request)) {
	app.mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if !permission.IsAdmin(ctx) {
			he.SendErrorToHTTPClient(w, "authorize", he.HTTPCodedErrorf(http.StatusUnauthorized, "permission denied"))
			return
		}
		id, err := idPathValue(w, r)
		if err != nil {
			he.SendErrorToHTTPClient(w, "parse url", err)
		}
		handler(ctx, id, w, r)
	})
}

func (app *irataApp) renderTournament(ctx context.Context, id int64, w http.ResponseWriter, _ *http.Request) {
	sc, err := app.siteStorage.FetchSiteConfig(ctx)
	if err != nil {
		he.SendErrorToHTTPClient(w, "fetch site config", err)
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
		Theme                   string
	}{
		Tournament:              t,
		InstallKeyboardHandlers: permission.CheckWriteAccessToTournamentID(ctx, id) == nil,
		Theme:                   sc.Theme,
	}
	log.Printf("render with args: %+v", args)
	if err := app.templates.ExecuteTemplate(w, "view-tournament.html.tmpl", args); err != nil {
		log.Printf("can't render template: %v", err)
	}
}

func (app *irataApp) installHandlers() {

	app.mux.HandleFunc("/robots.txt", handlers.HandleRobotsTXT)

	app.requiringAdminHandleFunc("/manage/structure", func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		// Fetch all structures
		slugs, err := app.appStorage.FetchStructureSlugs(ctx, 0, 100)
		if err != nil {
			he.SendErrorToHTTPClient(w, "fetch structure slugs", err)
			return
		}
		structures := []*model.Structure{}
		for _, slug := range slugs {
			st, err := app.appStorage.FetchStructure(ctx, slug.ID)
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

	app.requiringAdminHandleFunc("/create/tournament", func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		var flash string
		if r.Method == http.MethodPost {
			if err := r.ParseForm(); err != nil {
				log.Printf("error parsing form: %v", err)
				flash = "Error parsing form"
			} else if id, err := app.mutator.CreateTournament(ctx, r.Form); err != nil {
				log.Printf("error parsing form: %v", err)
				flash = "Error parsing form"
			} else {
				// success!
				http.Redirect(w, r, fmt.Sprintf("/t/%d", id), http.StatusSeeOther)
				return
			}
		}
		// Fetch available structures and footer plug sets
		structures, err := app.appStorage.FetchStructureSlugs(ctx, 0, 100)
		if err != nil {
			he.SendErrorToHTTPClient(w, "fetch structure slugs", err)
			return
		}
		footers, err := app.appStorage.ListFooterPlugSets(ctx)
		if err != nil {
			he.SendErrorToHTTPClient(w, "fetch footer plug sets", err)
			return
		}
		sc, err := app.siteStorage.FetchSiteConfig(ctx)
		if err != nil {
			he.SendErrorToHTTPClient(w, "fetch site config", err)
			return
		}
		paytables, err := app.paytableStorage.FetchPaytableSlugs(ctx)
		if err != nil {
			he.SendErrorToHTTPClient(w, "fetch paytable slugs", err)
			return
		}
		data := &editTournamentArgs{
			Structures: structures,
			FooterSets: footers,
			Flash:      flash,
			IsNew:      true,
			IsAdmin:    permission.IsAdmin(ctx),
			SiteConfig: sc,
			Tournament: &model.Tournament{State: &model.State{
				AutoComputePrizePool: true,
			}},
			Paytables: paytables,
		}
		if err := app.templates.ExecuteTemplate(w, "edit-tournament.html.tmpl", data); err != nil {
			log.Printf("can't render edit-tournament template: %v", err)
		}
	})

	app.requiringAdminTakingIDHandleFunc("/manage/structure/{id}/edit", func(ctx context.Context, id int64, w http.ResponseWriter, r *http.Request) {
		var flash string
		if r.Method == http.MethodPost {
			if err := r.ParseForm(); err != nil {
				log.Printf("error parsing form: %v", err)
				flash = "Error parsing form"
			} else {
				name := r.FormValue("Name")
				levels := []*model.Level{}
				for i := 0; ; i++ {
					durStr := r.FormValue(fmt.Sprintf("Level%dDuration", i))
					desc := r.FormValue(fmt.Sprintf("Level%dDescription", i))
					banner := r.FormValue(fmt.Sprintf("Level%dBanner", i))
					isBreak := r.FormValue(fmt.Sprintf("Level%dIsBreak", i)) == "on"
					if durStr == "" && desc == "" && banner == "" && !isBreak && i > 0 {
						break
					}
					if durStr == "" && desc == "" && banner == "" {
						continue
					}
					dur, err := strconv.Atoi(durStr)
					if err != nil || dur <= 0 || desc == "" || banner == "" {
						flash = "All fields required for each level"
						continue
					}
					levels = append(levels, &model.Level{
						DurationMinutes: dur,
						Description:     desc,
						IsBreak:         isBreak,
						Banner:          banner,
					})
				}
				if name == "" || len(levels) == 0 {
					flash = "Structure name and at least one level required"
				} else {
					st, err := app.appStorage.FetchStructure(ctx, id)
					if err != nil {
						flash = "Error fetching structure"
					} else {
						st.Name = name
						st.Levels = levels
						err := app.appStorage.SaveStructure(ctx, st)
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
		st, err := app.appStorage.FetchStructure(ctx, id)
		if err != nil {
			he.SendErrorToHTTPClient(w, "fetch structure", err)
			return
		}
		data := struct {
			Structure *model.Structure
			Flash     string
			IsNew     bool
		}{Structure: st, Flash: flash, IsNew: false}
		if err := app.templates.ExecuteTemplate(w, "edit-structure.html.tmpl", data); err != nil {
			log.Printf("can't render edit-structure template: %v", err)
		}
	})

	app.requiringAdminTakingIDHandleFunc("/manage/footer-set/{id}/edit", func(ctx context.Context, id int64, w http.ResponseWriter, r *http.Request) {
		var flash string
		if r.Method == http.MethodPost {
			if err := r.ParseForm(); err != nil {
				log.Printf("error parsing form: %v", err)
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
					err := app.appStorage.UpdateFooterPlugSet(ctx, id, name, plugs)
					if err != nil {
						flash = "Error saving footer plug set"
					} else {
						http.Redirect(w, r, fmt.Sprintf("/manage/footer-set/%d/edit", id), http.StatusSeeOther)
						return
					}
				}
			}
		}
		fp, err := app.appStorage.FetchPlugs(ctx, id)
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

	app.requiringAdminHandleFunc("/create/structure", func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		var flash string

		// For POST, handle the form submission
		if r.Method == http.MethodPost {
			if err := r.ParseForm(); err != nil {
				log.Printf("error parsing form: %v", err)
				flash = "Error parsing form"
			} else {
				name := r.FormValue("Name")
				levels := []*model.Level{}
				for i := 0; ; i++ {
					durStr := r.FormValue(fmt.Sprintf("Level%dDuration", i))
					desc := r.FormValue(fmt.Sprintf("Level%dDescription", i))
					banner := r.FormValue(fmt.Sprintf("Level%dBanner", i))
					isBreak := r.FormValue(fmt.Sprintf("Level%dIsBreak", i)) == "on"
					if durStr == "" && desc == "" && banner == "" && !isBreak && i > 0 {
						break
					}
					if durStr == "" && desc == "" && banner == "" {
						continue
					}
					dur, err := strconv.Atoi(durStr)
					if err != nil || dur <= 0 || desc == "" || banner == "" {
						flash = "All fields required for each level"
						continue
					}
					levels = append(levels, &model.Level{
						DurationMinutes: dur,
						Description:     desc,
						IsBreak:         isBreak,
						Banner:          banner,
					})
				}
				if name == "" || len(levels) == 0 {
					flash = "Structure name and at least one level required"
				} else {
					st := &model.Structure{
						StructureData: model.StructureData{
							Levels: levels,
						},
						Name: name,
					}
					_, err := app.appStorage.CreateStructure(ctx, st)
					if err != nil {
						flash = "Error saving structure"
					} else {
						http.Redirect(w, r, "/manage/structure", http.StatusSeeOther)
						return
					}
				}
			}
		}

		// Handle template ID from query param for pre-populating
		templateID := r.URL.Query().Get("template")
		var structure *model.Structure
		if templateID != "" {
			if id, err := strconv.ParseInt(templateID, 10, 64); err == nil {
				if st, err := app.appStorage.FetchStructure(ctx, id); err == nil {
					// Clone the structure but clear its ID
					structure = &model.Structure{
						StructureData: model.StructureData{
							Levels:        st.Levels,
							ChipsPerBuyIn: st.ChipsPerBuyIn,
							ChipsPerAddOn: st.ChipsPerAddOn,
						},
						Name: st.Name + " (Copy)",
					}
				}
			}
		}

		// If no template or error loading template, create empty structure
		if structure == nil {
			structure = &model.Structure{
				StructureData: model.StructureData{
					Levels: []*model.Level{},
				},
				Name: "",
			}
		}

		data := struct {
			Structure *model.Structure
			Flash     string
			IsNew     bool
		}{
			Structure: structure,
			Flash:     flash,
			IsNew:     true,
		}

		if err := app.templates.ExecuteTemplate(w, "edit-structure.html.tmpl", data); err != nil {
			log.Printf("can't render edit-structure template: %v", err)
		}
	})

	app.requiringAdminTakingIDHandleFunc("/manage/structure/{id}/delete", func(ctx context.Context, id int64, w http.ResponseWriter, r *http.Request) {
		err := app.appStorage.DeleteStructure(ctx, id)
		if err != nil {
			he.SendErrorToHTTPClient(w, "delete structure", err)
			return
		}
		http.Redirect(w, r, "/manage/structure", http.StatusSeeOther)
	})

	app.requiringAdminHandleFunc("/create/footer-set", func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		var flash string
		if r.Method == http.MethodPost {
			if err := r.ParseForm(); err != nil {
				log.Printf("error parsing form: %v", err)
				flash = "Error parsing form"
			} else {
				name := r.FormValue("Name")
				plugsRaw := r.FormValue("Plugs")
				if name == "" || plugsRaw == "" {
					flash = "All fields required"
				} else {
					plugs := parseFooterPlugsBox(plugsRaw)
					id, err := app.appStorage.CreateFooterPlugSet(ctx, name, plugs)
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

	app.requiringAdminTakingIDHandleFunc("/manage/footer-set/{id}/delete", func(ctx context.Context, id int64, w http.ResponseWriter, r *http.Request) {
		err := app.appStorage.DeleteFooterPlugSet(ctx, id)
		if err != nil {
			he.SendErrorToHTTPClient(w, "delete footer plug set", err)
			return
		}
		http.Redirect(w, r, "/manage/footer-set", http.StatusSeeOther)
	})

	app.requiringAdminHandleFunc("/manage/footer-set/", func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		sets, err := app.appStorage.ListFooterPlugSets(ctx)
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

	app.handleFunc("/", func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		sc, err := app.siteStorage.FetchSiteConfig(ctx)
		if err != nil {
			he.SendErrorToHTTPClient(w, "fetch site config", err)
			return
		}

		// TODO: pagination
		o, err := app.appStorage.FetchOverview(ctx, 0, 100)
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

	app.handleFunc("/favicon.ico", func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		http.ServeFileFS(w, r, app.subFS, "favicon.ico")
	})

	app.handleFunc("/logout", func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		app.bakery.ClearCookie(w)
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})

	// anything in fs is a file trivially shared
	app.mux.Handle("/fs/", http.StripPrefix("/fs/", http.FileServer(http.FS(app.subFS))))

	app.handleFuncTakingID("/t/{id}", func(ctx context.Context, id64 int64, w http.ResponseWriter, r *http.Request) {
		app.renderTournament(ctx, id64, w, r)
	})

	app.requiringAdminTakingIDHandleFunc("/t/{id}/delete", func(ctx context.Context, id64 int64, w http.ResponseWriter, r *http.Request) {
		if err := app.appStorage.DeleteTournament(ctx, id64); err != nil {
			he.SendErrorToHTTPClient(w, "delete tournament", err)
		} else {
			http.Redirect(w, r, "/", http.StatusPermanentRedirect)
		}
	})

	app.requiringAdminTakingIDHandleFunc("/t/{id}/edit", func(ctx context.Context, id64 int64, w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		if len(r.Form) != 0 {
			err := app.mutator.EditTournament(ctx, id64, r.Form)
			if err != nil {
				he.SendErrorToHTTPClient(w, "parse form", err)
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

		// Fetch structures and footer sets for the edit form
		structures, err := app.appStorage.FetchStructureSlugs(ctx, 0, 100)
		if err != nil {
			he.SendErrorToHTTPClient(w, "fetch structure slugs", err)
			return
		}
		footers, err := app.appStorage.ListFooterPlugSets(ctx)
		if err != nil {
			he.SendErrorToHTTPClient(w, "fetch footer plug sets", err)
			return
		}
		paytables, err := app.paytableStorage.FetchPaytableSlugs(ctx)
		if err != nil {
			he.SendErrorToHTTPClient(w, "fetch paytable slugs", err)
			return
		}

		sc, err := app.siteStorage.FetchSiteConfig(ctx)
		if err != nil {
			he.SendErrorToHTTPClient(w, "fetch site config", err)
			return
		}
		args := &editTournamentArgs{
			Tournament: t,
			Structures: structures,
			FooterSets: footers,
			Paytables:  paytables,
			IsAdmin:    permission.IsAdmin(ctx),
			IsNew:      false,
			SiteConfig: sc,
		}

		if err := app.templates.ExecuteTemplate(w, "edit-tournament.html.tmpl", args); err != nil {
			// don't use a.can't here, it would be a duplicate write to w
			log.Printf("500: can't render template: %v", err)
		}
	})

	app.handleFuncTakingID("/api/footerPlugs/{id}", func(ctx context.Context, id int64, w http.ResponseWriter, r *http.Request) {
		fp, err := app.appStorage.FetchPlugs(ctx, id)
		if err != nil {
			he.SendErrorToHTTPClient(w, "get plugs from db", err)
			return
		}

		for i, s := range fp.TextPlugs {
			fp.TextPlugs[i] = textutil.WrapLinesInNOBR(html.EscapeString(s))
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

	app.handleFuncTakingID("/api/model/{id}", func(ctx context.Context, id64 int64, w http.ResponseWriter, r *http.Request) {
		t, err := app.fetchTournament(ctx, id64)
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

	app.handleFunc("/login", func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
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
	app.handleFunc("/api/tournament-listen", func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		type reqBody struct {
			TournamentID  int64
			Version       int64
			ServerVersion int64
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
		version := req.Version
		if req.ServerVersion != model.ServerVersion {
			// trash the version number, we will need an update immediately and the client will
			// have to reload
			version = -1
		}
		go app.appStorage.ListenTournamentVersion(ctx, req.TournamentID, version, errCh, tournamentCh)
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

	app.requiringAdminHandleFunc("/manage/site", func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		var flash string
		// Fetch config
		config, err := readSiteConfig(ctx, app.siteStorage)
		if err != nil {
			he.SendErrorToHTTPClient(w, "fetch site config", err)
			return
		}

		if r.Method == http.MethodPost {
			if err := r.ParseForm(); err != nil {
				log.Printf("error parsing form: %v", err)
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

func ifb[T any](cond bool, t T, f T) T {
	if cond {
		return t
	}
	return f
}

func if10(b bool) int {
	return ifb(b, 10, 1)
}

func if10min(b bool) time.Duration {
	return ifb(b, 10*time.Minute, 1*time.Minute)
}

func makeKeyboardHandlers(clock ts.Clock, fetcher model.PaytableFetcher) map[string]func(*model.Tournament, *modifiers) error {
	// todo: it is bogus that these require a deps.  it would make more sense if these methods
	// were moved outside the model, since they are not just data, but actual actions.
	deps := &model.Deps{
		Clock:           clock,
		PaytableFetcher: fetcher,
	}

	return map[string]func(t *model.Tournament, bb *modifiers) error{
		"PreviousLevel": func(t *model.Tournament, bb *modifiers) error { return t.PreviousLevel(deps) },
		"SkipLevel":     func(t *model.Tournament, bb *modifiers) error { return t.AdvanceLevel(deps) },
		"StopClock":     func(t *model.Tournament, bb *modifiers) error { return t.StopClock(deps) },
		"StartClock":    func(t *model.Tournament, bb *modifiers) error { return t.StartClock(deps) },
		"RemovePlayer":  func(t *model.Tournament, bb *modifiers) error { return t.ChangePlayers(deps, -if10(bb.Shift)) },
		"AddPlayer":     func(t *model.Tournament, bb *modifiers) error { return t.ChangePlayers(deps, if10(bb.Shift)) },
		"AddBuyIn":      func(t *model.Tournament, bb *modifiers) error { return t.ChangeBuyIns(deps, if10(bb.Shift)) },
		"AddAddOn":      func(t *model.Tournament, bb *modifiers) error { return t.ChangeAddOns(deps, if10(bb.Shift)) },
		"RemoveAddOn":   func(t *model.Tournament, bb *modifiers) error { return t.ChangeAddOns(deps, -if10(bb.Shift)) },
		"RemoveBuyIn":   func(t *model.Tournament, bb *modifiers) error { return t.ChangeBuyIns(deps, -if10(bb.Shift)) },
		"PlusMinute":    func(t *model.Tournament, bb *modifiers) error { return t.PlusTime(deps, if10min(bb.Shift)) },
		"MinusMinute":   func(t *model.Tournament, bb *modifiers) error { return t.MinusTime(deps, if10min(bb.Shift)) },
	}
}

func (app *irataApp) handleKeypress(ctx context.Context, r *http.Request) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("can't read response body: %v", err)
	}

	type KeyboardModifyEvent struct {
		TournamentID int64
		Event        string
		Shift        bool
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
		t, err := app.fetchTournament(ctx, event.TournamentID)
		if err != nil {
			return he.HTTPCodedErrorf(404, "tournament not found: %w", err)
		}

		if err := h(t, &modifiers{Shift: event.Shift}); err != nil {
			return he.HTTPCodedErrorf(500, "while applying keyboard event: %w", err)
		}

		if err := app.appStorage.SaveTournament(ctx, t); err != nil {
			return he.HTTPCodedErrorf(500, "save tournament after keypress: %w", err)
		}
	}
	return nil
}

func (app *irataApp) installKeyboardHandlers() {
	app.requiringAdminHandleFunc("/api/keyboard-control", func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		err := app.handleKeypress(ctx, r)
		if err != nil {
			he.SendErrorToHTTPClient(w, "handleKeypress", err)
		}
	})

	app.requiringAdminHandleFunc("/api/prizePoolCalculator", func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			he.SendErrorToHTTPClient(w, "method not allowed", he.HTTPCodedErrorf(http.StatusMethodNotAllowed, "only POST allowed"))
			return
		}

		// Parse JSON request body
		var req struct {
			PaytableID             int64 `json:"paytableId"`
			BuyIns                 int   `json:"buyIns"`
			AddOns                 int   `json:"addOns"`
			Saves                  int   `json:"saves"`
			AmountPerSave          int   `json:"amountPerSave"`
			PrizePoolPerBuyIn      int   `json:"prizePoolPerBuyIn"`
			PrizePoolPerAddOn      int   `json:"prizePoolPerAddOn"`
			TotalPrizePoolOverride int   `json:"totalPrizePoolOverride"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			he.SendErrorToHTTPClient(w, "decode request", he.HTTPCodedErrorf(http.StatusBadRequest, "invalid JSON: %w", err))
			return
		}

		// Create a temporary tournament with the parameters
		tempTournament := &model.Tournament{
			PrizePoolPerBuyIn: req.PrizePoolPerBuyIn,
			PrizePoolPerAddOn: req.PrizePoolPerAddOn,
			State: &model.State{
				BuyIns:                 req.BuyIns,
				AddOns:                 req.AddOns,
				Saves:                  req.Saves,
				AmountPerSave:          req.AmountPerSave,
				TotalPrizePoolOverride: req.TotalPrizePoolOverride,
			},
		}

		// Compute the prize pool text
		prizePoolText, err := tempTournament.ComputePrizePoolText(app.paytableStorage)
		if err != nil {
			log.Printf("error computing prize pool: %v", err)
			he.SendErrorToHTTPClient(w, "compute prize pool", err)
			return
		}

		// Return the formatted text
		response := struct {
			PrizePoolText string `json:"prizePoolText"`
		}{
			PrizePoolText: prizePoolText,
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("error encoding response: %v", err)
		}
	})
}

func (app *irataApp) loadTemplates() {
	var err error
	if app.templates, err = template.New("root").Funcs(templateFuncs).ParseFS(assets.Templates, "templates/*[^~]"); err != nil {
		log.Fatalf("error loading embedded templates: %v", err)
	}
	for _, tmpl := range app.templates.Templates() {
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
		ch <- &result{"http", http.ListenAndServe(viper.GetString("listen_address"), app.handler)}
		wg.Done()
	}()

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

	config.Init()

	clock := ts.NewRealClock()
	subFS, err := fs.Sub(assets.FS, "fs")
	if err != nil {
		log.Fatalf("fs.Sub: %v", err)
	}

	unprotectedStorage, err := state.NewDBStorage(context.Background(), viper.GetString("db_url"), clock)
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

	mutator := action.New(storage, clock)

	csp := http.NewCrossOriginProtection()

	paytableStorage := state.NewDefaultPaytableStorage()

	app := &irataApp{
		appStorage:      storage,
		siteStorage:     unprotectedStorage,
		paytableStorage: paytableStorage,
		userStorage:     unprotectedStorage,
		mutator:         mutator,
		subFS:           subFS,
		bakery:          bakery,
		clock:           clock,
		modelDeps:       &model.Deps{Clock: clock, PaytableFetcher: paytableStorage},
	}

	app.mux = http.NewServeMux()

	// Stack the handlers together.  This isn't pretty.
	c2c := c2ctx.Handler(&c2ctx.Config{
		Bakery:      app.bakery,
		UserStorage: app.userStorage,
		Next:        app.mux,
	})
	logger := middleware.NewRequestLogger(csp.Handler(c2c), app.clock)
	tarpit := labrea.Handler(&labrea.Config{
		// Use real clock here for sub-ms precision.
		Clock: clockwork.NewRealClock(),
		Next:  logger,
	})
	app.handler = tarpit
	app.keypressHandlers = makeKeyboardHandlers(clock, paytableStorage)
	app.listenAddress = viper.GetString("listen_address")

	app.loadTemplates()
	app.installHandlers()

	if err := app.serve(); err != nil {
		log.Fatalf("can't serve: %v", err)
	}
}

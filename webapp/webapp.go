package webapp

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/ts4z/irata/app/handlers"
	"github.com/ts4z/irata/assets"
	"github.com/ts4z/irata/form"
	"github.com/ts4z/irata/he"
	"github.com/ts4z/irata/middleware"
	"github.com/ts4z/irata/middleware/c2ctx"
	"github.com/ts4z/irata/middleware/labrea"
	"github.com/ts4z/irata/model"
	"github.com/ts4z/irata/password"
	"github.com/ts4z/irata/paytable"
	"github.com/ts4z/irata/permission"
	"github.com/ts4z/irata/protocol"
	"github.com/ts4z/irata/soundmodel"
	"github.com/ts4z/irata/state"
	"github.com/ts4z/irata/textutil"
	"github.com/ts4z/irata/tournament"
	"github.com/ts4z/irata/urlpath"
	"github.com/ts4z/irata/webapp/ksd"
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

type editTournamentArgs struct {
	Flash      string
	Tournament *model.Tournament
	Structures []*model.StructureSlug
	FooterSets []*model.FooterPlugs
	Paytables  []*paytable.PaytableSlug
	IsAdmin    bool
	IsNew      bool
	SiteConfig *model.SiteConfig
	Sounds     []*soundmodel.SoundEffectSlug
}

// Config holds the configuration for creating a new IrataApp.
type Config struct {
	AppStorage        state.AppStorage
	SiteStorage       state.SiteStorage
	UserStorage       state.UserStorage
	PaytableStorage   state.PaytableStorage
	SoundStorage      state.SoundEffectStorage
	FormProcessor     *form.FormProcessor
	SubFS             fs.FS
	Bakery            *permission.Bakery
	Clock             nower
	TournamentMutator *tournament.Mutator
}

// App is the main web application.
type App struct {
	// storage
	templates *template.Template
	subFS     fs.FS

	// dependencies
	appStorage      state.AppStorage
	siteStorage     state.SiteStorage
	userStorage     state.UserStorage
	paytableStorage state.PaytableStorage
	soundStorage    state.SoundEffectStorage
	formProcessor   *form.FormProcessor
	bakery          *permission.Bakery
	clock           nower
	tm              *tournament.Mutator

	// internals
	mux     *http.ServeMux
	handler http.Handler
}

// New creates a new IrataApp with the given configuration.
func New(config *Config) *App {
	app := &App{
		appStorage:      config.AppStorage,
		siteStorage:     config.SiteStorage,
		userStorage:     config.UserStorage,
		paytableStorage: config.PaytableStorage,
		soundStorage:    config.SoundStorage,
		formProcessor:   config.FormProcessor,
		subFS:           config.SubFS,
		bakery:          config.Bakery,
		clock:           config.Clock,
		tm:              config.TournamentMutator,
		mux:             http.NewServeMux(),
	}

	// Stack the handlers together.
	c2c := c2ctx.Handler(&c2ctx.Config{
		Bakery:      app.bakery,
		UserStorage: app.userStorage,
		Next:        app.mux,
	})
	csp := http.NewCrossOriginProtection()
	logger := middleware.NewRequestLogger(csp.Handler(c2c), app.clock)
	tarpit := labrea.Handler(&labrea.Config{
		// Use real clock here for sub-ms precision.
		Clock: clockwork.NewRealClock(),
		Next:  logger,
	})
	app.handler = tarpit

	app.loadTemplates()
	app.InstallHandlers()

	return app
}

// Handler returns the configured HTTP handler.
func (app *App) Handler() http.Handler {
	return app.handler
}

func (app *App) fetchTournament(ctx context.Context, id int64) (*model.Tournament, error) {
	return app.appStorage.FetchTournament(ctx, id)
}

var regexpLFLF = regexp.MustCompile(`\n\n+`)

func parseFooterPlugsBox(plugsRaw string) []string {
	plugs := []string{}
	plugsRaw = strings.ReplaceAll(plugsRaw, "\r", "")
	for _, plug := range regexpLFLF.Split(plugsRaw, -1) {
		plug = strings.TrimSpace(plug)
		if plug != "" {
			plugs = append(plugs, plug)
		}
	}
	return plugs
}

func (app *App) handleFunc(pattern string, handler func(context.Context, http.ResponseWriter, *http.Request)) {
	app.mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		handler(ctx, w, r)
	})
}

func (app *App) handleFuncTakingID(pattern string, handler func(context.Context, int64, http.ResponseWriter, *http.Request)) {
	app.handleFunc(pattern, func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		id, err := idPathValue(w, r)
		if err != nil {
			he.SendErrorToHTTPClient(w, "parse url", err)
		}
		handler(ctx, id, w, r)
	})
}

func (app *App) requiringAdminHandleFunc(pattern string, handler func(context.Context, http.ResponseWriter, *http.Request)) {
	app.handleFunc(pattern, func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		if !permission.IsAdmin(ctx) {
			he.SendErrorToHTTPClient(w, "authorize", he.HTTPCodedErrorf(http.StatusUnauthorized, "permission denied"))
			return
		}
		handler(ctx, w, r)
	})
}

func (app *App) requiringAdminTakingIDHandleFunc(pattern string, handler func(context.Context, int64, http.ResponseWriter, *http.Request)) {
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

func (app *App) renderTournament(ctx context.Context, id int64, w http.ResponseWriter, _ *http.Request) {
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

func (app *App) handleManageStructure(ctx context.Context, w http.ResponseWriter, r *http.Request) {
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
}

func (app *App) handleCreateTournament(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	var flash string
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			log.Printf("error parsing form: %v", err)
			flash = "Error parsing form"
		} else if id, err := app.formProcessor.CreateTournament(ctx, r.Form); err != nil {
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
	sounds, err := app.soundStorage.FetchSoundEffectSlugs(ctx)
	if err != nil {
		he.SendErrorToHTTPClient(w, "fetch sound slugs", err)
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
		Sounds:    sounds,
	}
	if err := app.templates.ExecuteTemplate(w, "edit-tournament.html.tmpl", data); err != nil {
		log.Printf("can't render edit-tournament template: %v", err)
	}
}

func (app *App) handleEditStructure(ctx context.Context, id int64, w http.ResponseWriter, r *http.Request) {
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
}

func (app *App) handleEditFooterSet(ctx context.Context, id int64, w http.ResponseWriter, r *http.Request) {
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
}

func (app *App) handleCreateStructure(ctx context.Context, w http.ResponseWriter, r *http.Request) {
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
}

func (app *App) handleCreateFooterSet(ctx context.Context, w http.ResponseWriter, r *http.Request) {
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
}

func (app *App) handleManageFooterSets(ctx context.Context, w http.ResponseWriter, r *http.Request) {
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
}

func (app *App) handleIndex(ctx context.Context, w http.ResponseWriter, r *http.Request) {
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
}

func (app *App) handleEditTournament(ctx context.Context, id64 int64, w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	if len(r.Form) != 0 {
		err := app.formProcessor.EditTournament(ctx, id64, r.Form)
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
	sounds, err := app.soundStorage.FetchSoundEffectSlugs(ctx)
	if err != nil {
		he.SendErrorToHTTPClient(w, "fetch sound slugs", err)
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
		Sounds:     sounds,
	}

	if err := app.templates.ExecuteTemplate(w, "edit-tournament.html.tmpl", args); err != nil {
		// don't use a.can't here, it would be a duplicate write to w
		log.Printf("500: can't render template: %v", err)
	}
}

func (app *App) handleAPIFooterPlugs(ctx context.Context, id int64, w http.ResponseWriter, r *http.Request) {
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
}

func (app *App) handleAPIModel(ctx context.Context, id64 int64, w http.ResponseWriter, r *http.Request) {
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
}

func (app *App) handleLogin(ctx context.Context, w http.ResponseWriter, r *http.Request) {
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
}

func (app *App) handleAPITournamentListen(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	type reqBody struct {
		TournamentID    int64
		Version         int64
		ProtocolVersion int64
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
	if req.ProtocolVersion != protocol.Version {
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
}

func (app *App) handleManageSite(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	var flash string
	// Fetch config
	config, err := app.siteStorage.FetchSiteConfig(ctx)
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

	soundSlugs, err := app.soundStorage.FetchSoundEffectSlugs(ctx)
	if err != nil {
		he.SendErrorToHTTPClient(w, "fetch sound slugs", err)
		return
	}

	data := struct {
		Config *model.SiteConfig
		Sounds []*soundmodel.SoundEffectSlug
		Flash  string
	}{Config: config, Flash: flash, Sounds: soundSlugs}
	if err := app.templates.ExecuteTemplate(w, "manage-site.html.tmpl", data); err != nil {
		log.Printf("can't render manage-site template: %v", err)
	}
}

func (app *App) handleKeyboardControl(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	handler := ksd.NewKeyboardShortcutDispatcher(app.tm, app.appStorage)
	err := handler.HandleKeypress(ctx, r)
	if err != nil {
		log.Printf("error handling keypress: %v", err)
		he.SendErrorToHTTPClient(w, "handle keypress", err)
	}
}

func (app *App) handlePrizePoolCalculator(ctx context.Context, w http.ResponseWriter, r *http.Request) {
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
		PaytableID:        req.PaytableID,
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
	prizePoolText, err := app.tm.ComputePrizePoolText(tempTournament)
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
}

// InstallHandlers registers all HTTP routes.
func (app *App) InstallHandlers() {

	app.handleFunc("/", app.handleIndex)

	app.handleFunc("/favicon.ico", func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		http.ServeFileFS(w, r, app.subFS, "favicon.ico")
	})

	app.handleFunc("/logout", func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		app.bakery.ClearCookie(w)
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})

	app.handleFunc("/robots.txt", handlers.HandleRobotsTXT)

	// anything in fs is a file trivially shared
	app.mux.Handle("/fs/", http.StripPrefix("/fs/", http.FileServer(http.FS(app.subFS))))

	app.requiringAdminHandleFunc("/api/keyboard-control", app.handleKeyboardControl)

	app.requiringAdminHandleFunc("/manage/structure", app.handleManageStructure)

	app.requiringAdminHandleFunc("/create/tournament", app.handleCreateTournament)

	app.requiringAdminTakingIDHandleFunc("/manage/structure/{id}/edit", app.handleEditStructure)

	app.requiringAdminTakingIDHandleFunc("/manage/footer-set/{id}/edit", app.handleEditFooterSet)

	app.requiringAdminHandleFunc("/create/structure", app.handleCreateStructure)

	app.requiringAdminTakingIDHandleFunc("/manage/structure/{id}/delete", func(ctx context.Context, id int64, w http.ResponseWriter, r *http.Request) {
		err := app.appStorage.DeleteStructure(ctx, id)
		if err != nil {
			he.SendErrorToHTTPClient(w, "delete structure", err)
			return
		}
		http.Redirect(w, r, "/manage/structure", http.StatusSeeOther)
	})

	app.requiringAdminHandleFunc("/create/footer-set", app.handleCreateFooterSet)

	app.requiringAdminTakingIDHandleFunc("/manage/footer-set/{id}/delete", func(ctx context.Context, id int64, w http.ResponseWriter, r *http.Request) {
		err := app.appStorage.DeleteFooterPlugSet(ctx, id)
		if err != nil {
			he.SendErrorToHTTPClient(w, "delete structure", err)
			return
		}
		http.Redirect(w, r, "/manage/footer-set", http.StatusSeeOther)
	})

	app.requiringAdminHandleFunc("/manage/footer-set/", app.handleManageFooterSets)

	app.handleFuncTakingID("/t/{id}", app.renderTournament)

	app.requiringAdminTakingIDHandleFunc("/t/{id}/delete", func(ctx context.Context, id64 int64, w http.ResponseWriter, r *http.Request) {
		if err := app.appStorage.DeleteTournament(ctx, id64); err != nil {
			he.SendErrorToHTTPClient(w, "delete tournament", err)
		} else {
			http.Redirect(w, r, "/", http.StatusPermanentRedirect)
		}
	})

	app.requiringAdminTakingIDHandleFunc("/t/{id}/edit", app.handleEditTournament)

	app.handleFuncTakingID("/api/footerPlugs/{id}", app.handleAPIFooterPlugs)

	app.handleFuncTakingID("/api/model/{id}", app.handleAPIModel)

	app.handleFunc("/login", app.handleLogin)

	app.handleFunc("/api/tournament-listen", app.handleAPITournamentListen)

	app.requiringAdminHandleFunc("/api/prizePoolCalculator", app.handlePrizePoolCalculator)

	app.requiringAdminHandleFunc("/manage/site", app.handleManageSite)
}

func (app *App) loadTemplates() {
	var err error
	if app.templates, err = template.New("root").Funcs(templateFuncs).ParseFS(assets.Templates, "templates/*[^~]"); err != nil {
		log.Fatalf("error loading embedded templates: %v", err)
	}
	for _, tmpl := range app.templates.Templates() {
		log.Printf("loaded template %q", tmpl.Name())
	}
}

// Serve starts the HTTP server on the given listen address.
func (app *App) Serve(listenAddress string) error {
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

package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
	"net/http"

	"github.com/ts4z/irata/assets"
	"github.com/ts4z/irata/model"
	"github.com/ts4z/irata/storage"
	"github.com/ts4z/irata/textutil"
)

var templates *template.Template

var templateFuncs template.FuncMap = template.FuncMap{
	"wrapLinesInNOBR": textutil.WrapLinesInNOBR,
	"joinNLNL":        textutil.JoinNLNL,
}

func init() {
	var err error
	if templates, err = template.New("root").Funcs(templateFuncs).ParseFS(assets.Templates, "templates/*[^~]"); err != nil {
		log.Fatalf("error loading embedded templates: %v", err)
	}
	for _, tmpl := range templates.Templates() {
		// tmpl.Funcs(templateFuncs)
		log.Printf("loaded template %q", tmpl.Name())
	}
}

func blitSpecialFile(w http.ResponseWriter, filename string) {
	rf, err := assets.Special.Open(filename)
	if err != nil {
		log.Fatalf("can't open special file: %v", err)
	}
	_, err = io.Copy(w, rf)
	if err != nil {
		log.Printf("error copying special static file to client: %v",
			err)
	}
}

type KeyboardModifyEvent struct {
	Event string
}

var keyboardModifyEventHandlers = map[string]func(*model.Tournament) error{
	"PreviousLevel": func(t *model.Tournament) error { return t.PreviousLevel() },
	"SkipLevel":     func(t *model.Tournament) error { return t.SkipLevel() },
	"StopClock":     func(t *model.Tournament) error { return t.StopClock() },
	"StartClock":    func(t *model.Tournament) error { return t.StartClock() },
	"RemovePlayer":  func(t *model.Tournament) error { return t.RemovePlayer() },
	"AddPlayer":     func(t *model.Tournament) error { return t.AddPlayer() },
	"AddBuyIn":      func(t *model.Tournament) error { return t.AddBuyIn() },
	"RemoveBuyIn":   func(t *model.Tournament) error { return t.RemoveBuyIn() },
	"PlusMinute":    func(t *model.Tournament) error { return t.PlusMinute() },
	"MinusMinute":   func(t *model.Tournament) error { return t.MinusMinute() },
}

func main() {
	sub, err := fs.Sub(assets.FS, "fs")
	if err != nil {
		log.Fatalf("fs.Sub: %v", err)
	}

	http.HandleFunc("/",
		func(w http.ResponseWriter, r *http.Request) {
			o, err := storage.FetchOverview()
			if err != nil {
				log.Printf("500: can't fetch overview: %v", err)
				http.Error(w, fmt.Sprintf("can't fetch overview: %v", err), 500)
				return
			}
			if err := templates.ExecuteTemplate(w, "slash.html.tmpl", o); err != nil {
				log.Printf("500: can't render template: %v", err)
				http.Error(w, fmt.Sprintf("can't render template: %v", err), 500)
				return
			}
		})

	// anything in fs is a file trivially shared
	http.Handle("/fs/", http.StripPrefix("/fs/", http.FileServer(http.FS(sub))))

	http.HandleFunc("/event/", func(w http.ResponseWriter, r *http.Request) {
		// All events are the one event
		t, err := storage.FetchTournamentForView(1) // todo: ID goes here
		if err != nil {
			log.Printf("404: can't get tournament from database")
			http.Error(w, "can't get tournament from database", 404)
			return
		}
		if err := templates.ExecuteTemplate(w, "view.html.tmpl", t); err != nil {
			log.Printf("500: can't render template: %v", err)
			http.Error(w, fmt.Sprintf("can't render template: %v", err), 500)
		}
	})

	http.HandleFunc("/edit/event/1", func(w http.ResponseWriter, r *http.Request) {
		// All events are the one event
		t, err := storage.FetchTournament(1) // todo: ID goes here
		if err != nil {
			log.Printf("404: can't get tournament from database")
			http.Error(w, "can't get tournament from database", 404)
			return
		}
		if err := templates.ExecuteTemplate(w, "edit.html.tmpl", t); err != nil {
			log.Printf("500: can't render template: %v", err)
			http.Error(w, fmt.Sprintf("can't render template: %v", err), 500)
		}
	})

	http.HandleFunc("/api/model/", func(w http.ResponseWriter, r *http.Request) {
		t, err := storage.FetchTournament(1) // todo: ID goes here
		if err != nil {
			log.Printf("500: can't get tourney from db")
			http.Error(w, "can't get tournament from database", 500)
			return
		}
		bytes, err := json.Marshal(t)
		if err != nil {
			log.Printf("500: can't marshall")
			http.Error(w, fmt.Sprintf("can't marshal state model: %v", err), 500)
			return
		}
		writ, err := w.Write(bytes)
		if err != nil {
			log.Printf("error writing model to client: %v", err)
		} else if writ != len(bytes) {
			log.Println("short write to client")
		}
	})

	http.HandleFunc("/api/keyboard-control/", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			log.Printf("can't read response body: %v", err)
		}

		var event KeyboardModifyEvent
		if err := json.Unmarshal(body, &event); err != nil {
			log.Printf("can't unmarshal event %s: %v", string(body), err)
		}

		if h, ok := keyboardModifyEventHandlers[event.Event]; !ok {
			http.Error(w, "unknown event", 404)
		} else {
			t, err := storage.FetchTournament(1) // todo: ID goes here
			if err != nil {
				http.Error(w, fmt.Sprintf("tournament not found %v", err), 404)
				return
			}

			if err := h(t); err != nil {
				log.Printf("500: error handling keyboard event %v", err)
				http.Error(w, fmt.Sprintf("error in handler %q: %v", event.Event, err), 500)
			}
		}

	})
	log.Fatal(http.ListenAndServe(":8888", nil))
}

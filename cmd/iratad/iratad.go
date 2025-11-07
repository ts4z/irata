package main

import (
	"context"
	"io/fs"
	"log"

	"github.com/spf13/viper"

	"github.com/ts4z/irata/assets"
	"github.com/ts4z/irata/config"
	"github.com/ts4z/irata/dbcache"
	"github.com/ts4z/irata/form"
	"github.com/ts4z/irata/listener"
	"github.com/ts4z/irata/permission"
	"github.com/ts4z/irata/state"
	"github.com/ts4z/irata/tournament"
	"github.com/ts4z/irata/ts"
	"github.com/ts4z/irata/webapp"
)

func main() {
	ctx := context.Background()
	config.Init()

	clock := ts.NewRealClock()
	subFS, err := fs.Sub(assets.FS, "fs")
	if err != nil {
		log.Fatalf("fs.Sub: %v", err)
	}

	soundStorage := state.NewBuiltInSoundStorage()

	tournamentManager := tournament.NewManager(clock, state.NewDefaultPaytableStorage(), soundStorage)

	unprotectedStorage, err := state.NewDBStorage(context.Background(), viper.GetString("db_url"))
	if err != nil {
		log.Fatalf("can't configure database: %v", err)
	}
	defer unprotectedStorage.Close()

	siteConfigStorage := dbcache.NewSiteConfigCache(unprotectedStorage, clock)

	bakeryFactory := permission.NewBakeryFactory(clock, siteConfigStorage)
	if err != nil {
		log.Fatalf("can't create bakery: %v", err)
	}

	appStorage := &permission.AppStorage{
		Storage: dbcache.NewAppStorage(16, unprotectedStorage),
	}
	tournamentStorage := &permission.TournamentStorage{
		Storage: listener.NewTournamentStorage(
			dbcache.NewTournamentStorage(128, unprotectedStorage),
			tournamentManager),
	}

	userStorage := permission.NewUserStorage(
		dbcache.NewUserStorage(128, unprotectedStorage))

	mutator := form.NewProcessor(appStorage, tournamentStorage, userStorage, tournamentManager, clock)

	paytableStorage := state.NewDefaultPaytableStorage()

	app := webapp.New(ctx, &webapp.Config{
		AppStorage:        appStorage,
		TournamentStorage: tournamentStorage,
		SiteStorage:       unprotectedStorage,
		PaytableStorage:   paytableStorage,
		SoundStorage:      soundStorage,
		UserStorage:       userStorage,
		FormProcessor:     mutator,
		SubFS:             subFS,
		BakeryFactory:     bakeryFactory,
		Clock:             clock,
		TournamentManager: tournamentManager,
	})

	if err := app.Serve(ctx, viper.GetString("listen_address")); err != nil {
		log.Fatalf("can't serve: %v", err)
	}
}

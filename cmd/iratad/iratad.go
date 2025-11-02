package main

import (
	"context"
	"io/fs"
	"log"

	"github.com/spf13/viper"

	"github.com/ts4z/irata/assets"
	"github.com/ts4z/irata/config"
	"github.com/ts4z/irata/form"
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

	sefs := state.NewBuiltInSoundStorage()

	tournamentMutator := tournament.NewMutator(clock, state.NewDefaultPaytableStorage(), sefs)

	unprotectedStorage, err := state.NewDBStorage(context.Background(), viper.GetString("db_url"), tournamentMutator)
	if err != nil {
		log.Fatalf("can't configure database: %v", err)
	}

	siteConfig, err := unprotectedStorage.FetchSiteConfig(ctx)
	if err != nil {
		log.Fatalf("can't fetch site config: %v", err)
	}

	bakery, err := permission.New(clock, siteConfig)
	if err != nil {
		log.Fatalf("can't create bakery: %v", err)
	}

	storage := &permission.StoragePermissionFacade{Storage: unprotectedStorage}

	mutator := form.New(storage, tournamentMutator)

	paytableStorage := state.NewDefaultPaytableStorage()

	soundStorage := state.NewBuiltInSoundStorage()

	app := webapp.New(ctx, &webapp.Config{
		AppStorage:        storage,
		SiteStorage:       unprotectedStorage,
		PaytableStorage:   paytableStorage,
		SoundStorage:      soundStorage,
		UserStorage:       unprotectedStorage,
		FormProcessor:     mutator,
		SubFS:             subFS,
		Bakery:            bakery,
		Clock:             clock,
		TournamentMutator: tournamentMutator,
	})

	if err := app.Serve(ctx, viper.GetString("listen_address")); err != nil {
		log.Fatalf("can't serve: %v", err)
	}
}

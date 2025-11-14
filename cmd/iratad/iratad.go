package main

import (
	"context"
	"io/fs"
	"log"

	"github.com/spf13/viper"

	"github.com/ts4z/irata/assets"
	"github.com/ts4z/irata/config"
	"github.com/ts4z/irata/dbcache"
	"github.com/ts4z/irata/dbnotify"
	"github.com/ts4z/irata/dbutil"
	"github.com/ts4z/irata/form"
	"github.com/ts4z/irata/gossip"
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

	db, err := dbutil.Connect()
	if err != nil {
		log.Fatalf("can't connect to database: %v", err)
	}

	unprotectedStorage, err := state.NewDBStorage(context.Background(), db)
	if err != nil {
		log.Fatalf("can't configure database: %v", err)
	}
	defer unprotectedStorage.Close()

	cachedSiteConfigStorage := dbcache.NewSiteConfigStorage(unprotectedStorage, clock)
	siteStorageReader := permission.NewSiteConfigStorageReader(cachedSiteConfigStorage)
	protectedSiteConfigStorage := permission.NewSiteConfigStorage(cachedSiteConfigStorage)

	bakeryFactory := permission.NewBakeryFactory(clock, cachedSiteConfigStorage)
	if err != nil {
		log.Fatalf("can't create bakery: %v", err)
	}

	appStorage := &permission.AppStorage{
		Storage: dbcache.NewAppStorage(16, unprotectedStorage),
	}
	cachedTournamentStorage := dbcache.NewTournamentStorage(128, unprotectedStorage)
	tournamentGossiper := gossip.NewTournamentGossiper(cachedTournamentStorage, tournamentManager)
	tournamentStorage := &permission.TournamentStorage{
		Storage: gossip.NewTournamentStorage(
			cachedTournamentStorage,
			tournamentGossiper),
	}

	cachedUserStorage := dbcache.NewUserStorage(128, unprotectedStorage)
	userStorage := permission.NewUserStorage(cachedUserStorage)

	userGossiper := gossip.NewUserGossiper(cachedUserStorage)

	userDispatcher := dbnotify.NewChangeDispatcher("users", userGossiper, cachedUserStorage, cachedUserStorage)

	mutator := form.NewProcessor(appStorage, tournamentStorage, userStorage, tournamentManager, clock)

	paytableStorage := state.NewDefaultPaytableStorage()

	// TODO: This doesn't look right.

	tourneyDispatcher := dbnotify.NewChangeDispatcher("tournaments",
		tournamentGossiper, cachedTournamentStorage, cachedTournamentStorage)

	// TODO: site config dispatcher, footer plug dispatcher, etc.

	dbListener, err := dbnotify.NewDBNotifyListener(db, tourneyDispatcher, userDispatcher)
	if err != nil {
		log.Fatalf("can't create db notificationlistener: %v", err)
	}

	app := webapp.New(ctx, &webapp.Config{
		TournamentGossiper: tournamentGossiper,
		DBListener:         dbListener,
		AppStorage:         appStorage,
		TournamentStorage:  tournamentStorage,
		SiteStorage:        protectedSiteConfigStorage,
		SiteStorageReader:  siteStorageReader,
		PaytableStorage:    paytableStorage,
		SoundStorage:       soundStorage,
		UserStorage:        userStorage,
		FormProcessor:      mutator,
		SubFS:              subFS,
		BakeryFactory:      bakeryFactory,
		Clock:              clock,
		TournamentManager:  tournamentManager,
	})

	if err := app.Serve(ctx, viper.GetString("listen_address")); err != nil {
		log.Fatalf("can't serve: %v", err)
	}
}

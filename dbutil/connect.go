package dbutil

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net"
	"os"

	"cloud.google.com/go/cloudsqlconn"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/ts4z/irata/config"
)

type cloudEnvSettings struct {
	dbUser,
	dbPwd,
	dbName,
	instanceConnectionName,
	usePrivate string
}

func (s *cloudEnvSettings) getenv() error {
	unset := []string{}
	getenv := func(k string) string {
		v := os.Getenv(k)
		if v == "" {
			unset = append(unset, k)
		}
		return v
	}

	s.dbUser = getenv("DB_USER")                                  // e.g. 'my-db-user'
	s.dbPwd = getenv("DB_PASS")                                   // e.g. 'my-db-password'
	s.dbName = getenv("DB_NAME")                                  // e.g. 'my-database'
	s.instanceConnectionName = getenv("INSTANCE_CONNECTION_NAME") // e.g. 'project:region:instance'
	s.usePrivate = os.Getenv("PRIVATE_IP")

	if unset != nil {
		return fmt.Errorf("cloudsqlconn: unset variables: %+v", unset)
	}
	return nil
}

func connectWithConnector() (*sql.DB, error) {
	// Note: Saving credentials in environment variables is convenient, but not
	// secure - consider a more secure solution such as
	// Cloud Secret Manager (https://cloud.google.com/secret-manager) to help
	// keep passwords and other secrets safe.

	env := &cloudEnvSettings{}
	env.getenv()

	dsn := fmt.Sprintf("user=%s password=%s database=%s", env.dbUser, env.dbPwd, env.dbName)
	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	var opts []cloudsqlconn.Option
	if env.usePrivate != "" {
		opts = append(opts, cloudsqlconn.WithDefaultDialOptions(cloudsqlconn.WithPrivateIP()))
	}
	// WithLazyRefresh() Option is used to perform refresh
	// when needed, rather than on a scheduled interval.
	// This is recommended for serverless environments to
	// avoid background refreshes from throttling CPU.
	opts = append(opts, cloudsqlconn.WithLazyRefresh())
	d, err := cloudsqlconn.NewDialer(context.Background(), opts...)
	if err != nil {
		return nil, err
	}
	// Use the Cloud SQL connector to handle connecting to the instance.
	// This approach does *NOT* require the Cloud SQL proxy.
	config.DialFunc = func(ctx context.Context, network, instance string) (net.Conn, error) {
		return d.Dial(ctx, env.instanceConnectionName)
	}
	dbURI := stdlib.RegisterConnConfig(config)
	dbPool, err := sql.Open("pgx", dbURI)
	if err != nil {
		return nil, fmt.Errorf("sql.Open: %w", err)
	}
	return dbPool, nil
}

func connectWithPgx() (*sql.DB, error) {
	url := config.DBURL()
	log.Printf("Connecting to database at %s", url)
	if url == "" {
		return nil, errors.New("database URL is empty")
	}
	return sql.Open("pgx", url)
}

func Connect() (*sql.DB, error) {
	factories := map[string]func() (*sql.DB, error){
		"connector": connectWithConnector,
		"pgx":       connectWithPgx,
	}
	factory, ok := factories[config.SQLConnector()]
	if !ok {
		log.Fatalf("unknown value for config.SQLConnector(): %q", config.SQLConnector())
	}
	return factory()
}

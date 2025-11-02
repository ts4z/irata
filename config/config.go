// Package config handles pre-database configuration, such as the location of the database.
// This is used by both irata and irataadmin.
//
// TODO: I have never seen a viper setup that I liked.
package config

import (
	"log"
	"os"

	"github.com/spf13/viper"
)

// Viper-based config loader
func Init() {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	viper.SetConfigType("yaml")
	viper.SetConfigName(".irata")
	viper.AddConfigPath(home)
	viper.AutomaticEnv()
	viper.BindEnv("db_url", "IRATA_DB_URL")
	viper.BindEnv("listen_address", "IRATA_LISTEN_ADDRESS")
	viper.BindEnv("sql_connector", "IRATA_SQL_CONNECTOR")
	viper.SetDefault("db_url", "")
	viper.SetDefault("listen_address", ":8080")
	viper.SetDefault("sql_connector", "dbx")
	err = viper.ReadInConfig() // ignore error if config file missing
	if err != nil {
		log.Printf("viper can't read config file: %v", err)
	}
	log.Printf("Using database URL: %s", viper.GetString("db_url"))
	log.Printf("Using listen address: %s", viper.GetString("listen_address"))
}

func DBURL() string {
	return viper.GetString("db_url")
}

func ListenAddress() string {
	return viper.GetString("listen_address")
}

func SecureCookies() bool {
	return viper.GetBool("secure_cookies")
}

func SQLConnector() string {
	return viper.GetString("sql_connector")
}

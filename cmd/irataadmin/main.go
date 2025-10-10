package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/spf13/cobra"

	"github.com/ts4z/irata/model"
	"github.com/ts4z/irata/state"
)

const (
	hashKeySize  = 32
	blockKeySize = 128
)

var (
	dbURL        string
	honorOffset  time.Duration
	mintDuration time.Duration
	startOffset  time.Duration
	clock        clockwork.Clock = clockwork.NewRealClock()
)

func generateKey(sz int) ([]byte, error) {
	key := make([]byte, sz)
	_, err := rand.Read(key)
	if err != nil {
		return nil, fmt.Errorf("generating random key: %w", err)
	}
	return key, nil
}

func getKeyStatus(now time.Time, v model.CookieKeyValidity) string {
	if now.Before(v.MintFrom) {
		return "not yet active"
	}
	if now.After(v.HonorUntil) {
		return "expired"
	}
	if now.After(v.MintUntil) {
		// it's an older code, but it checks out
		return "obsolete"
	}
	return "active"
}

func listKeys(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	storage, err := state.NewDBStorage(ctx, dbURL)
	if err != nil {
		return fmt.Errorf("connecting to database: %w", err)
	}
	defer storage.Close()

	config, err := storage.FetchSiteConfig(ctx)
	if err != nil {
		return fmt.Errorf("fetching site config: %w", err)
	}

	now := clock.Now()
	fmt.Printf("Current keys (as of %v):\n\n", now.Format(time.RFC3339))

	for i, key := range config.CookieKeys {
		fmt.Printf("Key %d:\n", i+1)
		fmt.Printf("  Mint window:  %v to %v\n",
			key.Validity.MintFrom.Format(time.RFC3339),
			key.Validity.MintUntil.Format(time.RFC3339))
		fmt.Printf("  Honor until: %v\n",
			key.Validity.HonorUntil.Format(time.RFC3339))
		fmt.Printf("  Status: %v\n\n",
			getKeyStatus(now, key.Validity))
	}
	return nil
}

func rotateKeys(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	storage, err := state.NewDBStorage(ctx, dbURL)
	if err != nil {
		return fmt.Errorf("connecting to database: %w", err)
	}
	defer storage.Close()

	config, err := storage.FetchSiteConfig(ctx)
	if err != nil {
		return fmt.Errorf("fetching site config: %w", err)
	}

	now := clock.Now()
	validKeys := []model.CookieKeyPair{}

	// Keep only non-expired keys
	for _, key := range config.CookieKeys {
		if now.Before(key.Validity.HonorUntil) {
			validKeys = append(validKeys, key)
		}
	}

	hashKey, err := generateKey(hashKeySize)
	if err != nil {
		return fmt.Errorf("generating hash key: %w", err)
	}

	blockKey, err := generateKey(blockKeySize)
	if err != nil {
		return fmt.Errorf("generating block key: %w", err)
	}

	mintFrom := now.Add(startOffset)
	mintUntil := mintFrom.Add(mintDuration)
	honorUntil := mintUntil.Add(honorOffset)

	newKey := model.CookieKeyPair{
		Validity: model.CookieKeyValidity{
			MintFrom:   mintFrom,
			MintUntil:  mintUntil,
			HonorUntil: honorUntil,
		},
		HashKey64:  base64.StdEncoding.EncodeToString(hashKey),
		BlockKey64: base64.StdEncoding.EncodeToString(blockKey),
	}

	config.CookieKeys = append(validKeys, newKey)

	if err := storage.SaveSiteConfig(ctx, config); err != nil {
		return fmt.Errorf("saving updated config: %w", err)
	}

	fmt.Printf("Key rotation complete:\n")
	fmt.Printf("  Start minting: %v\n", mintFrom.Format(time.RFC3339))
	fmt.Printf("  Stop minting:  %v\n", mintUntil.Format(time.RFC3339))
	fmt.Printf("  Honor until:   %v\n", honorUntil.Format(time.RFC3339))
	return nil
}

func main() {
	rootCmd := &cobra.Command{
		Short: "Irata administration tool",
		Use:   "irataadmin",
	}
	rootCmd.PersistentFlags().StringVar(&dbURL, "db", "postgresql:///irata", "Database connection URL")

	keyCmd := &cobra.Command{
		Short: "Manage authentication keys",
		Use:   "key",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List current keys and their status",
		RunE:  listKeys,
	}

	rotateCmd := &cobra.Command{
		Use:   "rotate",
		Short: "Remove expired keys and add a new key",
		RunE:  rotateKeys,
	}
	rotateCmd.Flags().DurationVar(&startOffset, "start-offset", 0, "How long to wait before the key becomes valid (e.g. 24h)")
	rotateCmd.Flags().DurationVar(&mintDuration, "mint-duration", 180*24*time.Hour, "How long the key should be valid for minting (default 180 days)")
	rotateCmd.Flags().DurationVar(&honorOffset, "honor-offset", 180*24*time.Hour, "How long after minting ends to honor the key (default 180 days)")

	keyCmd.AddCommand(listCmd, rotateCmd)
	rootCmd.AddCommand(keyCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

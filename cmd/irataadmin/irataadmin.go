package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"syscall"
	"text/tabwriter"
	"time"

	"golang.org/x/term"

	"github.com/jonboulle/clockwork"
	"github.com/spf13/cobra"
	"maze.io/x/duration"

	"github.com/ts4z/irata/config"
	"github.com/ts4z/irata/model"
	"github.com/ts4z/irata/password"
	"github.com/ts4z/irata/state"
)

const (
	// these sizes are recommended by the gorilla/securecookie package
	// https://pkg.go.dev/github.com/gorilla/securecookie#New
	hashKeySize  = 32
	blockKeySize = 16
)

var (
	honorOffset  time.Duration
	mintDuration time.Duration
	startOffset  time.Duration
	clock        clockwork.Clock = clockwork.NewRealClock()

	userNick    string
	userEmail   string
	userIsAdmin bool

	expireTime time.Time
)

func newUserStorage(ctx context.Context) state.UserStorage {
	config.Init()
	storage, err := state.NewDBStorage(ctx, config.DBURL(), clock)
	if err != nil {
		log.Fatalf("can't connect to database: %v", err)
	}
	return storage
}

func newSiteStorage(ctx context.Context) state.SiteStorage {
	storage, err := state.NewDBStorage(ctx, config.DBURL(), clock)
	if err != nil {
		log.Fatalf("can't connect to database: %v", err)
	}
	return storage
}

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
	storage := newSiteStorage(ctx)
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
	storage := newSiteStorage(ctx)
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

func addUser(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	storage := newUserStorage(ctx)

	defer storage.Close()

	if userNick == "" || userEmail == "" {
		return fmt.Errorf("name and email are required")
	}

	fmt.Print("Enter password: ")
	pwBytes, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		return fmt.Errorf("reading password: %w", err)
	}
	userPassword := string(pwBytes)
	if userPassword == "" {
		return fmt.Errorf("password is required")
	}

	hashedPassword := password.Hash(userPassword)

	err = storage.CreateUser(ctx, userNick, userEmail, hashedPassword, userIsAdmin)
	if err != nil {
		return fmt.Errorf("creating user: %w", err)
	}

	fmt.Printf("User %q added successfully.\n", userNick)
	return nil
}

func listUsers(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	storage := newUserStorage(ctx)
	defer storage.Close()

	users, err := storage.FetchUsers(ctx)
	if err != nil {
		return fmt.Errorf("fetching users: %w", err)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)

	fmt.Fprintf(w, "id\tnick\tadmin\n")
	for _, user := range users {
		fmt.Fprintf(w, "%d\t%s\t%v\n", user.ID, user.Nick, user.IsAdmin)
	}
	w.Flush()
	return nil
}

func checkPassword(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	storage := newUserStorage(ctx)
	defer storage.Close()

	nick := args[0]

	fmt.Print("Enter password: ")
	pwBytes, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		return fmt.Errorf("reading password: %w", err)
	}
	userPassword := string(pwBytes)
	if userPassword == "" {
		return fmt.Errorf("password is required")
	}

	userRow, err := storage.FetchUserRow(ctx, nick)
	if err != nil {
		return fmt.Errorf("fetching user %q: %w", nick, err)
	}

	checker, err := password.NewChecker(clock, userRow)
	if err != nil {
		return fmt.Errorf("setting up password checker: %w", err)
	}

	_, err = checker.Validate(userPassword)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		return nil
	}

	fmt.Printf("ok\n")
	return nil
}

func deleteUser(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	storage := newUserStorage(ctx)
	defer storage.Close()

	nick := args[0]

	err := storage.DeleteUserByNick(ctx, nick)
	if err != nil {
		return fmt.Errorf("deleting user %q: %w", nick, err)
	}

	return nil
}

func cleanPasswords(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	storage := newUserStorage(ctx)
	defer storage.Close()

	return storage.RemoveExpiredPasswords(ctx, clock.Now())
}

func addPassword(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	storage := newUserStorage(ctx)
	defer storage.Close()

	nick := args[0]

	userRow, err := storage.FetchUserRow(ctx, nick)
	if err != nil {
		return fmt.Errorf("fetching user %q: %w", nick, err)
	}

	fmt.Print("Enter password: ")
	pwBytes, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		return fmt.Errorf("reading password: %w", err)
	}
	userPassword := string(pwBytes)
	if userPassword == "" {
		return fmt.Errorf("password is required")
	}

	hashedPassword := password.Hash(userPassword)

	err = storage.AddPassword(ctx, userRow.ID, hashedPassword)
	if err != nil {
		return fmt.Errorf("adding password for user %q: %w", nick, err)
	}

	fmt.Printf("Password added successfully for user %q.\n", nick)
	return nil
}

func replacePassword(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	storage := newUserStorage(ctx)
	defer storage.Close()

	nick := args[0]

	userRow, err := storage.FetchUserRow(ctx, nick)
	if err != nil {
		return fmt.Errorf("fetching user %q: %v", nick, err)
	}

	fmt.Print("Enter new password: ")
	pwBytes, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		return fmt.Errorf("reading password: %w", err)
	}
	newPassword := string(pwBytes)
	if newPassword == "" {
		return fmt.Errorf("password is required")
	}

	hashedPassword := password.Hash(newPassword)

	err = storage.ReplacePassword(ctx, userRow.ID, hashedPassword, expireTime)
	if err != nil {
		return fmt.Errorf("replacing password for user %q: %w", nick, err)
	}

	fmt.Printf("Password replaced successfully for user %q. Old passwords expire at %v.\n", nick, expireTime)
	return nil
}

// ...existing code...

// ...existing code...

func main() {
	config.Init()

	rootCmd := &cobra.Command{
		Short: "Irata administration tool",
		Use:   "irataadmin",
	}

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

	userCmd := &cobra.Command{
		Short: "Manage users",
		Use:   "user",
	}

	addUserCmd := &cobra.Command{
		Use:   "add",
		Short: "Add a new user",
		RunE:  addUser,
	}
	addUserCmd.Flags().StringVar(&userNick, "nick", "", "User's nick")
	addUserCmd.Flags().StringVar(&userEmail, "email", "", "User's email address")
	addUserCmd.Flags().BoolVar(&userIsAdmin, "admin", false, "Set user as admin")

	deleteUserCmd := &cobra.Command{
		Use:   "delete [nick]",
		Short: "Delete a user",
		Args:  cobra.ExactArgs(1),
		RunE:  deleteUser,
	}

	listUserCmd := &cobra.Command{
		Use:   "list",
		Short: "List all users",
		RunE:  listUsers,
	}

	pwCmd := &cobra.Command{
		Use:   "pw",
		Short: "Password-related operations for users",
	}

	checkCmd := &cobra.Command{
		Use:   "check [nick]",
		Short: "Check a user's password",
		Args:  cobra.ExactArgs(1),
		RunE:  checkPassword,
	}

	clean := &cobra.Command{
		Use:   "clean",
		Short: "Remove expired passwords",
		RunE:  cleanPasswords,
	}

	addPassword := &cobra.Command{
		Use:   "add [nick]",
		Short: "Add a password for a user",
		Args:  cobra.ExactArgs(1),
		RunE:  addPassword,
	}

	replacePasswordCmd := &cobra.Command{
		Use:   "replace [nick]",
		Short: "Replace a user's password and expire old passwords",
		Args:  cobra.ExactArgs(1),
		RunE:  replacePassword,
	}
	replacePasswordCmd.Flags().Func("expire-time", "Expiration time for old passwords (RFC3339)", func(s string) error {
		if s == "" {
			expireTime = clock.Now()
			return nil
		}
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			expireTime = t
			return nil
		}
		if d, err := duration.ParseDuration(s); err == nil {
			expireTime = clock.Now().Add(time.Duration(d))
			return nil
		}

		return nil
	})
	pwCmd.AddCommand(checkCmd, clean, addPassword, replacePasswordCmd)
	userCmd.AddCommand(addUserCmd, listUserCmd, deleteUserCmd, pwCmd)
	rootCmd.AddCommand(userCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

package state

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/ts4z/irata/dbutil"
	"github.com/ts4z/irata/he"
	"github.com/ts4z/irata/model"
)

type Clock interface {
	Now() time.Time
}

// DBStorage stores stuff in the associated database.
//
// TODO: Split apart.  There is too much in one class, and the three
// interfaces are easily seperable.  (The database handle can
// be shared.)
type DBStorage struct {
	db    *sql.DB
	clock Clock
	// Map from tournament id to slice of notification functions
	tournamentListeners   map[int64][]chan<- *model.Tournament
	tournamentListenersMu sync.Mutex
}

var _ AppStorage = &DBStorage{}
var _ SiteStorage = &DBStorage{}
var _ UserStorage = &DBStorage{}

func NewDBStorage(ctx context.Context, url string, clock Clock) (*DBStorage, error) {
	db, err := sql.Open("pgx", url)
	if err != nil {
		return nil, err
	}
	return &DBStorage{
		clock:               clock,
		db:                  db,
		tournamentListeners: make(map[int64][]chan<- *model.Tournament),
	}, nil
}

func (s *DBStorage) Close() {
	s.db.Close()
}

func (s *DBStorage) FetchSiteConfig(ctx context.Context) (*model.SiteConfig, error) {
	rows, err := s.db.Query("SELECT key, value from site_info;")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	readSiteConfig := false
	config := &model.SiteConfig{}
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		switch key {
		case "conf":
			if readSiteConfig {
				return nil, errors.New("duplicate site config")
			}
			if err := json.Unmarshal([]byte(value), &config); err != nil {
				return nil, err
			} else {
				readSiteConfig = true
			}
		}
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}

	if !readSiteConfig {
		return nil, errors.New("no site config found")
	}

	return config, nil
}

func (s *DBStorage) FetchPlugs(ctx context.Context, id int64) (*model.FooterPlugs, error) {
	return nil, errors.ErrUnsupported
}

func (s *DBStorage) FetchStructure(ctx context.Context, id int64) (*model.Structure, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT version, model_data, name FROM structures where structure_id=$1", id)
	if err != nil {
		return nil, fmt.Errorf("querying structure: %w", err)
	}

	defer rows.Close()

	n := 0
	st := &model.Structure{}
	for rows.Next() {
		n++
		var name string
		var lock int64
		var bytes []byte

		if err := rows.Scan(&lock, &bytes, &name); err != nil {
			return nil, err
		}

		err := json.Unmarshal(bytes, st)
		if err != nil {
			return nil, err
		}

		st.Name = name
		st.ID = id
		st.Version = lock
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	if n != 1 {
		return nil, he.New(404, fmt.Errorf("no such structure id %d", id))
	}

	return st, nil
}

func (s *DBStorage) FetchStructureSlugs(ctx context.Context, offset, limit int) ([]*model.StructureSlug, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, name FROM structures LIMIT $1 OFFSET $2", limit, offset)
	if err != nil {
		return nil, fmt.Errorf("querying structures: %w", err)
	}

	defer rows.Close()

	slugs := []*model.StructureSlug{}
	for rows.Next() {
		var name string
		var id int64

		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}

		slugs = append(slugs, &model.StructureSlug{
			ID:   id,
			Name: name,
		})
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return slugs, nil
}

func (s *DBStorage) FetchOverview(ctx context.Context, offset, limit int) (*model.Overview, error) {
	rows, err := s.db.Query("SELECT tournament_id, model_data from tournaments LIMIT $1 OFFSET $2;", limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	overview := &model.Overview{}
	for rows.Next() {
		var id int64
		var bytes []byte

		if err := rows.Scan(&id, &bytes); err != nil {
			log.Printf("Row scan failed: %v", err)
			continue
		}
		// log.Printf("read: id=%d, bytes=%q", id, bytes)
		tournament := model.Tournament{}
		err := json.Unmarshal(bytes, &tournament)
		if err != nil {
			log.Printf("JSON unmarshal failed: %v", err)
			continue
		}
		slug := model.TournamentSlug{
			TournamentID:   id,
			TournamentName: tournament.EventName,
			Description:    tournament.Description,
		}

		overview.Slugs = append(overview.Slugs, slug)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return overview, nil
}

func (s *DBStorage) FetchTournament(ctx context.Context, id int64) (*model.Tournament, error) {
	var lock int64
	var handle string
	var bytes []byte

	err := s.db.QueryRowContext(ctx, `SELECT version, handle, model_data FROM tournaments where tournament_id=$1`, id).Scan(&lock, &handle, &bytes)

	if err == sql.ErrNoRows {
		return nil, he.New(404, fmt.Errorf("no such tournament id %d", id))
	} else if err != nil {
		return nil, he.New(500, fmt.Errorf("querying tournament: %w", err))
	}

	tm := &model.Tournament{}
	err = json.Unmarshal(bytes, tm)
	if err != nil {
		return nil, err
	}

	// These come from the database row, not the JSON.
	tm.EventID = id
	tm.Handle = handle
	tm.Version = lock
	// These don't come from the database at all.
	tm.FillTransients(s.clock)

	return tm, nil
}

func (s *DBStorage) CreateTournament(
	ctx context.Context,
	tm *model.Tournament) (int64, error) {

	cpy := *tm
	cpy.Transients = nil
	cpy.State.BuyIns = 0
	cpy.State.AddOns = 0
	cpy.State.CurrentPlayers = 0

	bytes, err := json.Marshal(&cpy)
	if err != nil {
		return 0, err
	}

	result, err := s.db.ExecContext(ctx, `INSERT INTO tournaments (handle, model_data) VALUES ($1, $2) RETURNING tournament_id;`,
		tm.Handle, bytes)
	if err != nil {
		return 0, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (s *DBStorage) SaveTournament(
	ctx context.Context,
	tm *model.Tournament) error {
	cpy := *tm

	cpy.Transients = nil
	bytes, err := json.Marshal(&cpy)
	if err != nil {
		return err
	}
	newVersion := tm.Version + 1
	if result, err := s.db.ExecContext(ctx,
		`UPDATE tournaments SET version=$4, model_data=$2 WHERE tournament_id=$3 AND version=$1;`,
		tm.Version,
		bytes,
		tm.EventID,
		newVersion); err != nil {
		log.Printf("update failed: %v", err)
		return err
	} else {
		if n, err := result.RowsAffected(); err != nil {
			return err
		} else if n != 1 {
			return fmt.Errorf("optimistic lock failure, %d rows affected", n)
		}
	}

	cpy.Version = newVersion
	cpy.FillTransients(s.clock)

	// Notify listeners for this tournament id
	var listeners []chan<- *model.Tournament
	s.tournamentListenersMu.Lock()
	listeners = s.tournamentListeners[tm.EventID]
	delete(s.tournamentListeners, tm.EventID)
	s.tournamentListenersMu.Unlock()

	for _, ch := range listeners {
		// Pass the updated tournament directly
		go func(ch chan<- *model.Tournament) {
			ch <- &cpy
		}(ch)
	}

	log.Printf("wrote: tournament id=%d version=%d notified %d listeners", tm.EventID, tm.Version, len(listeners))
	return nil
}

func (s *DBStorage) DeleteTournament(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx,
		"DELETE from tournaments WHERE tournament_id=$1", id)

	if err != nil {
		return err
	} else {
		if n, err := result.RowsAffected(); err != nil {
			return err
		} else if n != 1 {
			return fmt.Errorf("%d rows deleted", n)
		} else {
			return nil
		}
	}
}

func (s *DBStorage) SaveStructure(
	ctx context.Context,
	st *model.Structure) error {
	cpy := *st

	bytes, err := json.Marshal(&cpy)
	if err != nil {
		return err
	}
	if result, err := s.db.ExecContext(ctx,
		`UPDATE tournaments SET version=$1+1, name=$4, model_data=$2 WHERE structure_id=$3 AND version=$1;`,
		st.Version, bytes, st.ID, st.Name); err != nil {
		log.Printf("update failed: %v", err)
		return err
	} else {
		if n, err := result.RowsAffected(); err != nil {
			return err
		} else if n != 1 {
			return fmt.Errorf("optimistic lock failure, %d rows affected", n)
		}
	}

	return nil
}

func (s *DBStorage) DeleteStructure(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx,
		"DELETE from structures WHERE id=$1", id)

	if err != nil {
		return err
	} else {
		if n, err := result.RowsAffected(); err != nil {
			return err
		} else if n != 1 {
			return fmt.Errorf("%d rows deleted", n)
		} else {
			return nil
		}
	}
}

func (s *DBStorage) CreateStructure(
	ctx context.Context,
	st *model.Structure) (int64, error) {

	cpy := *st

	bytes, err := json.Marshal(&cpy)
	if err != nil {
		return 0, err
	}

	result, err := s.db.ExecContext(ctx,
		`INSERT INTO structures (name, model_data) 
		    VALUES ($1, $2) RETURNING structure_id;`,
		st.Name, bytes)
	if err != nil {
		return 0, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (s *DBStorage) SaveSiteConfig(ctx context.Context, config *model.SiteConfig) error {
	bytes, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	result, err := s.db.ExecContext(ctx,
		`UPDATE site_info SET value=$1 WHERE key='conf'`,
		bytes)
	if err != nil {
		return fmt.Errorf("updating site_info: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if rows != 1 {
		return fmt.Errorf("expected 1 row affected, got %d", rows)
	}

	return nil
}

func (s *DBStorage) CreateUser(ctx context.Context, nick string, emailAddress string, passwordHash string, isAdmin bool) error {
	tx, err := dbutil.NewTx(ctx, s.db, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.MaybeRollback()

	var userID int64
	// Insert into users table
	err = tx.QueryRow(ctx,
		`INSERT INTO users (is_admin, nick) VALUES ($1, $2) RETURNING user_id`,
		isAdmin, nick).Scan(&userID)
	if err != nil {
		return fmt.Errorf("insert users: %w", err)
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO user_email_addresses (email_address, user_id) VALUES ($1, $2)`,
		emailAddress, userID)
	if err != nil {
		return fmt.Errorf("insert user_email_addresses: %w", err)
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO passwords (user_id, hashed_password) VALUES ($1, $2)`,
		userID, passwordHash)
	if err != nil {
		return fmt.Errorf("insert passwords: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	return nil
}

// TODO: This is broken in the case of multiple passwords.
func (s *DBStorage) FetchUserRow(ctx context.Context, nick string) (*model.UserRow, error) {
	var row model.UserRow
	rows, err := s.db.QueryContext(ctx,
		`SELECT user_id, hashed_password, expires, is_admin, nick FROM users
		NATURAL JOIN passwords
		WHERE nick=$1;`,
		nick)
	if err != nil {
		log.Printf("error querying user row for %a: %v", err, nick)
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var hashed string
		var expires *time.Time
		var nick string
		var isAdmin bool
		if err := rows.Scan(&row.ID, &hashed, &expires, &isAdmin, &nick); err != nil {
			log.Printf("error scanning user row for %q: %v", nick, err)
			return nil, err
		}
		row.Passwords = append(row.Passwords, model.Password{
			PasswordHash: hashed,
			ExpiresAt:    expires,
		})
		row.Nick = nick
		row.IsAdmin = isAdmin
	}

	return &row, nil
}

func (s *DBStorage) FetchUserByUserID(ctx context.Context, id int64) (*model.UserIdentity, error) {
	row := &model.UserIdentity{}
	err := s.db.QueryRowContext(ctx,
		"SELECT nick, is_admin FROM users WHERE user_id=$1;", id).Scan(
		&row.Nick, &row.IsAdmin)
	if err == sql.ErrNoRows {
		return nil, errors.New("user not found")
	}
	if err != nil {
		return nil, err
	}
	return row, nil
}

func (s *DBStorage) FetchUsers(ctx context.Context) ([]*model.UserIdentity, error) {
	// TODO: pagination
	rows, err := s.db.QueryContext(ctx,
		`SELECT user_id, nick, is_admin FROM users ORDER BY user_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*model.UserIdentity
	for rows.Next() {
		var user model.UserIdentity
		if err := rows.Scan(&user.ID, &user.Nick, &user.IsAdmin); err != nil {
			return nil, err
		}
		users = append(users, &user)
	}
	return users, nil
}

func (s *DBStorage) DeleteUserByNick(ctx context.Context, nick string) error {
	result, err := s.db.ExecContext(ctx,
		"DELETE from users WHERE nick=$1", nick)

	if err != nil {
		return err
	} else {
		if n, err := result.RowsAffected(); err != nil {
			return err
		} else if n != 1 {
			return fmt.Errorf("%d rows deleted", n)
		} else {
			return nil
		}
	}
}

// ListenTournamentVersion registers a channel to be notified when the tournament version changes.
// If the version is already different, closes the channel immediately.
// If tournament not found, sends error on channel and closes it.
func (s *DBStorage) ListenTournamentVersion(ctx context.Context, id int64, clientVersion int64, errCh chan<- error, tournamentCh chan<- *model.Tournament) {
	var dbVersion int64
	err := s.db.QueryRowContext(ctx, "SELECT version FROM tournaments WHERE tournament_id=$1", id).Scan(&dbVersion)
	if err != nil {
		errCh <- err
		return
	}
	if dbVersion != clientVersion {
		tm, fetchErr := s.FetchTournament(ctx, id)
		if fetchErr != nil {
			errCh <- fetchErr
		} else {
			tournamentCh <- tm
		}
		return
	}

	s.tournamentListenersMu.Lock()
	s.tournamentListeners[id] = append(s.tournamentListeners[id], tournamentCh)
	log.Printf("%d listeners for tournament id %d", len(s.tournamentListeners[id]), id)
	s.tournamentListenersMu.Unlock()
}

// AddPassword adds a new password hash for a user.
func (s *DBStorage) AddPassword(ctx context.Context, userID int64, passwordHash string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO passwords (user_id, hashed_password) VALUES ($1, $2)`,
		userID, passwordHash)
	return err
}

// RemoveExpiredPasswords removes all passwords that expired before the given time.
func (s *DBStorage) RemoveExpiredPasswords(ctx context.Context, before time.Time) error {
	result, err := s.db.ExecContext(ctx,
		`DELETE FROM passwords WHERE expires IS NOT NULL AND expires < $1`,
		before)
	if err != nil {
		log.Printf("error removing expired passwords: %v", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		log.Printf("error checking rows affected removing expired passwords: %v", err)
	}
	fmt.Printf("%d expired\n", n)
	return err
}

// ReplacePassword replaces a user's password and expires old passwords at the given time.
func (s *DBStorage) ReplacePassword(ctx context.Context, userID int64, newPasswordHash string, oldPasswordsExpire time.Time) error {
	tx, err := dbutil.NewTx(ctx, s.db, nil)
	if err != nil {
		return err
	}
	defer tx.MaybeRollback()

	// Expire all current passwords for the user
	_, err = tx.Exec(ctx,
		`UPDATE passwords SET expires = $1 WHERE user_id = $2 AND (expires IS NULL OR expires > $1)`,
		oldPasswordsExpire, userID)
	if err != nil {
		return err
	}

	// Add the new password
	_, err = tx.Exec(ctx,
		`INSERT INTO passwords (user_id, hashed_password) VALUES ($1, $2)`,
		userID, newPasswordHash)
	if err != nil {
		return err
	}

	return tx.Commit()
}

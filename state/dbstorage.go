package state

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/ts4z/irata/dbutil"
	"github.com/ts4z/irata/he"
	"github.com/ts4z/irata/model"
	"github.com/ts4z/irata/varz"
)

const (
	ConfKey = 1
)

var (
	fetchFooterPlugs = varz.NewInt("fetchFooterPlugs")
	fetchUserByID    = varz.NewInt("fetchUserByID")
	fetchTournament  = varz.NewInt("fetchTournament")
	saveTournament   = varz.NewInt("saveTournament")
)

// DBStorage stores stuff in the associated database.
//
// TODO: Split apart.  There is too much in one class, and the three
// interfaces are easily seperable.  (The database handle can
// be shared.)
type DBStorage struct {
	db *sql.DB
	// Map from tournament id to slice of notification functions
	tournamentListeners   map[int64][]chan<- *model.Tournament
	tournamentListenersMu sync.Mutex
}

// FetchPlugs fetches a footer plug set and its plugs by set ID.
func (s *DBStorage) FetchPlugs(ctx context.Context, id int64) (*model.FooterPlugs, error) {
	fetchFooterPlugs.Add(1)
	row := s.db.QueryRowContext(ctx, `SELECT id, name, version FROM footer_plug_sets WHERE id = $1`, id)
	var setID, version int64
	var name string
	if err := row.Scan(&setID, &name, &version); err != nil {
		if err == sql.ErrNoRows {
			return nil, he.New(404, fmt.Errorf("no such footer plug set id %d", id))
		}
		return nil, err
	}
	plugs := []string{}
	rows, err := s.db.QueryContext(ctx, `SELECT text FROM text_footer_plugs WHERE footer_plug_set_id = $1 ORDER BY id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var text string
		if err := rows.Scan(&text); err != nil {
			return nil, err
		}
		plugs = append(plugs, text)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return &model.FooterPlugs{
		FooterPlugsID: setID,
		Version:       version,
		Name:          name,
		TextPlugs:     plugs,
	}, nil
}

// ListFooterPlugSets lists all footer plug sets (metadata only).
func (s *DBStorage) ListFooterPlugSets(ctx context.Context) ([]*model.FooterPlugs, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, version FROM footer_plug_sets ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	sets := []*model.FooterPlugs{}
	for rows.Next() {
		var setID, version int64
		var name string
		if err := rows.Scan(&setID, &name, &version); err != nil {
			return nil, err
		}
		sets = append(sets, &model.FooterPlugs{
			FooterPlugsID: setID,
			Version:       version,
			Name:          name,
			TextPlugs:     nil, // not loaded here
		})
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return sets, nil
}

// CreateFooterPlugSet creates a new footer plug set with a name and initial plugs.
func (s *DBStorage) CreateFooterPlugSet(ctx context.Context, name string, plugs []string) (int64, error) {
	tx, err := dbutil.NewTx(ctx, s.db, nil)
	if err != nil {
		return 0, err
	}
	defer tx.MaybeRollback()
	var setID int64
	err = tx.QueryRow(ctx, `INSERT INTO footer_plug_sets (name) VALUES ($1) RETURNING id`, name).Scan(&setID)
	if err != nil {
		return 0, err
	}
	for _, plug := range plugs {
		_, err := tx.Exec(ctx, `INSERT INTO text_footer_plugs (footer_plug_set_id, text) VALUES ($1, $2)`, setID, plug)
		if err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return setID, nil
}

// UpdateFooterPlugSet updates the name and plugs of a footer plug set.
func (s *DBStorage) UpdateFooterPlugSet(ctx context.Context, id int64, name string, plugs []string) error {
	tx, err := dbutil.NewTx(ctx, s.db, nil)
	if err != nil {
		return err
	}
	defer tx.MaybeRollback()
	_, err = tx.Exec(ctx, `UPDATE footer_plug_sets SET name = $1, version = version + 1 WHERE id = $2`, name, id)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `DELETE FROM text_footer_plugs WHERE footer_plug_set_id = $1`, id)
	if err != nil {
		return err
	}
	for _, plug := range plugs {
		_, err := tx.Exec(ctx, `INSERT INTO text_footer_plugs (footer_plug_set_id, text) VALUES ($1, $2)`, id, plug)
		if err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

// DeleteFooterPlugSet deletes a footer plug set and all its plugs.
func (s *DBStorage) DeleteFooterPlugSet(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM footer_plug_sets WHERE id = $1`, id)
	return err
}

var _ AppStorage = &DBStorage{}
var _ SiteStorage = &DBStorage{}
var _ SiteStorageReader = &DBStorage{}
var _ UserStorage = &DBStorage{}
var _ TournamentStorage = &DBStorage{}

func NewDBStorage(ctx context.Context, db *sql.DB) (*DBStorage, error) {
	return &DBStorage{
		db:                  db,
		tournamentListeners: make(map[int64][]chan<- *model.Tournament),
	}, nil
}

func (s *DBStorage) Close() {
	s.db.Close()
}

// TODO: Looks like this could be simpler since we only ever store one row.
func (s *DBStorage) FetchSiteConfig(ctx context.Context) (*model.SiteConfig, error) {
	rows, err := s.db.Query("SELECT id, value from site_config;")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	readSiteConfig := false
	config := &model.SiteConfig{}
	for rows.Next() {
		var id int64
		var value string
		if err := rows.Scan(&id, &value); err != nil {
			return nil, err
		}
		switch id {
		case ConfKey:
			if readSiteConfig {
				return nil, errors.New("duplicate site config")
			}
			if err := json.Unmarshal([]byte(value), &config); err != nil {
				return nil, err
			} else {
				readSiteConfig = true
			}
		default:
			log.Fatalf("unknown site_config key %d", id)
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
	rows, err := s.db.QueryContext(ctx, "SELECT structure_id, name, model_data FROM structures LIMIT $1 OFFSET $2", limit, offset)
	if err != nil {
		return nil, fmt.Errorf("querying structures: %w", err)
	}

	defer rows.Close()

	slugs := []*model.StructureSlug{}
	for rows.Next() {
		var name string
		var id int64
		var modelData []byte

		if err := rows.Scan(&id, &name, &modelData); err != nil {
			return nil, err
		}

		// Parse model_data to extract chip values
		var sd model.StructureData
		if err := json.Unmarshal(modelData, &sd); err != nil {
			return nil, fmt.Errorf("unmarshaling structure data: %w", err)
		}

		slugs = append(slugs, &model.StructureSlug{
			ID:            id,
			Name:          name,
			ChipsPerBuyIn: sd.ChipsPerBuyIn,
			ChipsPerAddOn: sd.ChipsPerAddOn,
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
	fetchTournament.Add(1)
	var lock int64
	var bytes []byte

	err := s.db.QueryRowContext(ctx, `SELECT version, model_data FROM tournaments where tournament_id=$1`, id).Scan(&lock, &bytes)

	if err == sql.ErrNoRows {
		return nil, he.New(404, fmt.Errorf("no such tournament id %d", id))
	} else if err != nil {
		return nil, he.New(500, fmt.Errorf("querying tournament: %w", err))
	}

	t := &model.Tournament{}
	err = json.Unmarshal(bytes, t)
	if err != nil {
		return nil, err
	}

	// These come from the database row, not the JSON.
	t.EventID = id
	t.Version = lock

	return t, nil
}

func (s *DBStorage) CreateTournament(
	ctx context.Context,
	t *model.Tournament) (int64, error) {

	if len(t.Structure.Levels) == 0 {
		return 0, fmt.Errorf("cannot create tournament with no structure levels")
	}

	var id int64

	cpy := *t
	cpy.State.IsClockRunning = false

	// Set level to full time.
	// q.v. Tournament.restartLevel
	millis := (time.Duration(t.CurrentLevel().DurationMinutes) * time.Minute).Milliseconds()
	cpy.State.TimeRemainingMillis = &millis

	bytes, err := json.Marshal(&cpy)
	if err != nil {
		return 0, err
	}

	if err := s.db.QueryRowContext(ctx, `INSERT INTO tournaments (model_data) VALUES ($1) RETURNING tournament_id;`,
		bytes).Scan(&id); err != nil {
		return 0, err
	}

	return id, nil
}

func (s *DBStorage) SaveTournament(
	ctx context.Context,
	tm *model.Tournament) error {

	saveTournament.Add(1)

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

	tm.Version = newVersion

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
		`UPDATE structures SET version=$1+1, name=$4, model_data=$2 WHERE structure_id=$3 AND version=$1;`,
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
		"DELETE from structures WHERE structure_id=$1", id)

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

	if err := s.db.QueryRowContext(ctx,
		`INSERT INTO structures (name, model_data) VALUES ($1, $2) RETURNING structure_id;`,
		st.Name, bytes).Scan(&st.ID); err != nil {
		log.Printf("insert structure failed: %v", err)
		return 0, err
	}

	return st.ID, nil
}

func (s *DBStorage) SaveSiteConfig(ctx context.Context, config *model.SiteConfig) error {
	bytes, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	result, err := s.db.ExecContext(ctx,
		`UPDATE site_config SET value=$1 WHERE id=$2`,
		bytes, ConfKey)
	if err != nil {
		return fmt.Errorf("updating site_config: %w", err)
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

func (s *DBStorage) CreateUser(ctx context.Context, u *model.UserIdentity) (int64, error) {
	var userID int64
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO users (nick, is_admin, is_operator) VALUES ($1, $2, $3) RETURNING user_id`,
		u.Nick, u.IsAdmin, u.IsOperator).Scan(&userID)
	if err != nil {
		return -1, he.HTTPCodedErrorf(500, "dadbase insert failed: %w", err)
	}
	return userID, nil
}

func (s *DBStorage) CreateUserWithEmailAndPassword(ctx context.Context, nick string, emailAddress string, passwordHash string, isAdmin bool) error {
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
	log.Printf("FetchUserRow nick=%q", nick)
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
	fetchUserByID.Add(1)
	row := &model.UserIdentity{ID: id}
	err := s.db.QueryRowContext(ctx,
		`SELECT nick, is_admin, is_operator
		FROM users
		WHERE user_id=$1`, id).Scan(
		&row.Nick, &row.IsAdmin, &row.IsOperator)
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
		`SELECT user_id, nick, is_admin, is_operator
		FROM users
		ORDER BY user_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*model.UserIdentity
	for rows.Next() {
		var user model.UserIdentity
		if err := rows.Scan(&user.ID, &user.Nick, &user.IsAdmin, &user.IsOperator); err != nil {
			return nil, err
		}
		users = append(users, &user)
	}
	return users, nil
}

func (s *DBStorage) SaveUser(ctx context.Context, u *model.UserIdentity) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE users SET nick=$1, is_admin=$2, is_operator=$3 WHERE user_id=$4`,
		u.Nick, u.IsAdmin, u.IsOperator, u.ID)
	if err != nil {
		return he.HTTPCodedErrorf(http.StatusInternalServerError, "db error: %w", err)
	}

	n, err := result.RowsAffected()
	if err != nil {
		return he.HTTPCodedErrorf(http.StatusInternalServerError, "db error: %w", err)
	}
	if n != 1 {
		return he.HTTPCodedErrorf(http.StatusNotFound, "no rows affected")
	}

	return nil
}

func (s *DBStorage) DeleteUserByID(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx,
		"DELETE from users WHERE user_id=$1", id)

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

	s.tournamentListenersMu.Lock()
	defer s.tournamentListenersMu.Unlock()

	if dbVersion != clientVersion {
		tm, fetchErr := s.FetchTournament(ctx, id)
		if fetchErr != nil {
			errCh <- fetchErr
		} else {
			tournamentCh <- tm
		}
		return
	}

	s.tournamentListeners[id] = append(s.tournamentListeners[id], tournamentCh)
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

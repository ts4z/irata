package password

import (
	"encoding/base64"
	"errors"
	"log" // all kids love log
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/ts4z/irata/model"
)

var (
	ErrInvalidPassword  = errors.New("invalid password")
	ErrNoPasswordHashes = errors.New("no password hashes available")
)

type private struct{}

type Clock interface {
	Now() time.Time
}

func Hash(pw string) string {
	bytes, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("can't hash password: %v", err)
	}
	return base64.RawStdEncoding.EncodeToString(bytes)
}

type Checker struct {
	private
	hashes   [][]byte
	identity *model.UserIdentity
}

func NewChecker(clock Clock, userRow *model.UserRow) (*Checker, error) {
	byteHashes := make([][]byte, 0, len(userRow.Passwords))
	for _, pw := range userRow.Passwords {
		if pw.ExpiresAt != nil && clock.Now().After(*pw.ExpiresAt) {
			continue
		}

		bytes, err := base64.RawStdEncoding.DecodeString(pw.PasswordHash)
		if err != nil {
			log.Printf("can't decode hashed password for %d %q: %v", userRow.ID, userRow.Nick, err)
			continue
		}
		byteHashes = append(byteHashes, bytes)
	}

	if len(byteHashes) == 0 {
		return nil, ErrNoPasswordHashes
	}

	return &Checker{private: private{}, hashes: byteHashes, identity: &userRow.UserIdentity}, nil
}

func (ch *Checker) Validate(pw string) (*model.UserIdentity, error) {
	errors := []error{}
	for _, hash := range ch.hashes {
		if err := bcrypt.CompareHashAndPassword(hash, []byte(pw)); err == nil {
			return ch.identity, nil
		} else {
			errors = append(errors, err)
		}
	}

	log.Printf("password.Checker.Validate failed for %v: errors: %+v", ch.identity.Nick, errors)

	return nil, ErrInvalidPassword
}

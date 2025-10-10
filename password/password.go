package password

import (
	"encoding/base64"
	"fmt"
	"log" // all kids love log

	"golang.org/x/crypto/bcrypt"

	"github.com/ts4z/irata/model"
)

type private struct{}

func Hash(pw string) string {
	bytes, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("can't hash password: %v", err)
	}
	return base64.RawStdEncoding.EncodeToString(bytes)
}

type Checker struct {
	private
	hash     []byte
	identity *model.UserIdentity
}

func NewChecker(userRow *model.UserRow) (*Checker, error) {
	bytes, err := base64.RawStdEncoding.DecodeString(userRow.PasswordHash)
	if err != nil {
		return nil, fmt.Errorf("can't decode hashed password: %w", err)
	}
	return &Checker{private: private{}, hash: bytes, identity: &userRow.UserIdentity}, nil
}

func (ch *Checker) Validate(pw string) (*model.UserIdentity, error) {
	if err := bcrypt.CompareHashAndPassword(ch.hash, []byte(pw)); err != nil {
		return nil, fmt.Errorf("invalid password")
	} else {
		return ch.identity, nil
	}
}

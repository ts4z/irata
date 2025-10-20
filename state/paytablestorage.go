package state

import (
	"github.com/ts4z/irata/defaults"
	"github.com/ts4z/irata/he"
	"github.com/ts4z/irata/paytable"
)

type DefaultPaytableStorage struct {
	paytables map[string]*paytable.Paytable
}

func NewDefaultPaytableStorage() *DefaultPaytableStorage {
	return &DefaultPaytableStorage{
		paytables: map[string]*paytable.Paytable{
			"BARGE Unified Poker Payouts": defaults.BARGEPaytable(),
		},
	}
}

func (d *DefaultPaytableStorage) Close() {
	// No resources to clean up
}

func (d *DefaultPaytableStorage) FetchPaytableByName(name string) (*paytable.Paytable, error) {
	if pt, ok := d.paytables[name]; ok {
		return pt, nil
	} else {
		return nil, he.HTTPCodedErrorf(404, "paytable not found")
	}
}

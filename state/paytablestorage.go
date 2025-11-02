package state

import (
	"context"

	"github.com/ts4z/irata/builtins"
	"github.com/ts4z/irata/he"
	"github.com/ts4z/irata/paytable"
)

var _ PaytableStorage = (*BuiltinPaytableStorage)(nil)

type BuiltinPaytableStorage struct {
	paytables map[string]*paytable.Paytable
}

// FetchPayoutTableByID implements PaytableStorage.
func (d *BuiltinPaytableStorage) FetchPaytableByID(ctx context.Context, id int64) (*paytable.Paytable, error) {
	for _, pt := range d.paytables {
		if pt.ID == id {
			return pt, nil
		}
	}
	return nil, he.HTTPCodedErrorf(404, "paytable not found")
}

// FetchPayoutTableSlugs implements PaytableStorage.
func (d *BuiltinPaytableStorage) FetchPaytableSlugs(ctx context.Context) ([]*paytable.PaytableSlug, error) {
	slugs := []*paytable.PaytableSlug{}
	for name, pt := range d.paytables {
		slugs = append(slugs, &paytable.PaytableSlug{
			Name: name,
			ID:   pt.ID,
		})
	}
	return slugs, nil
}

func NewDefaultPaytableStorage() *BuiltinPaytableStorage {
	return &BuiltinPaytableStorage{
		paytables: map[string]*paytable.Paytable{
			"BARGE Unified Poker Payouts": builtins.BARGEPaytable(),
		},
	}
}

func (d *BuiltinPaytableStorage) Close() {
	// No resources to clean up
}

func (d *BuiltinPaytableStorage) FetchPaytableByName(name string) (*paytable.Paytable, error) {
	if pt, ok := d.paytables[name]; ok {
		return pt, nil
	} else {
		return nil, he.HTTPCodedErrorf(404, "paytable not found")
	}
}

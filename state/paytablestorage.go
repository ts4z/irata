package state

import (
	"context"

	"github.com/ts4z/irata/defaults"
	"github.com/ts4z/irata/he"
	"github.com/ts4z/irata/paytable"
)

var _ PaytableStorage = (*DefaultPaytableStorage)(nil)

type DefaultPaytableStorage struct {
	paytables map[string]*paytable.Paytable
}

// FetchPayoutTableByID implements PaytableStorage.
func (d *DefaultPaytableStorage) FetchPayoutTableByID(ctx context.Context, id int64) (*paytable.Paytable, error) {
	for _, pt := range d.paytables {
		if pt.ID == id {
			return pt, nil
		}
	}
	return nil, he.HTTPCodedErrorf(404, "paytable not found")
}

// FetchPayoutTableSlugs implements PaytableStorage.
func (d *DefaultPaytableStorage) FetchPayoutTableSlugs(ctx context.Context) ([]*paytable.PaytableSlug, error) {
	slugs := []*paytable.PaytableSlug{}
	for name, pt := range d.paytables {
		slugs = append(slugs, &paytable.PaytableSlug{
			Name: name,
			ID:   pt.ID,
		})
	}
	return slugs, nil
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

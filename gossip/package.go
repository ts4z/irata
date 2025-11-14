/*
package gossip provides an interface between our clients and notifications from
our database as well as our local writes.

The name is imperfect, but see the section "Promotion" on https://en.wikipedia.org/wiki/Hadacol.
*/

package gossip

import "context"

type CacheStorage[T any] interface {
	Fetch(ctx context.Context, id int64) (*T, error)
	CacheInvalidate(ctx context.Context, key int64, version int64)
	// CacheStore(ctx context.Context, value *T)
}

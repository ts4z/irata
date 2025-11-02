package state

import (
	"context"

	"github.com/ts4z/irata/builtins"
	"github.com/ts4z/irata/he"
	"github.com/ts4z/irata/soundmodel"
)

var _ SoundEffectStorage = (*BuiltinSoundStorage)(nil)

type BuiltinSoundStorage struct {
	idToSoundEffect map[int64]*soundmodel.SoundEffect
}

func (bs *BuiltinSoundStorage) Close() {}

func NewBuiltInSoundStorage() *BuiltinSoundStorage {
	bs := &BuiltinSoundStorage{
		idToSoundEffect: map[int64]*soundmodel.SoundEffect{},
	}
	for _, se := range builtins.SoundEffects() {
		bs.idToSoundEffect[se.ID] = se
	}
	return bs
}

// FetchSoundEffectByID implements SoundStorage.
func (bs *BuiltinSoundStorage) FetchSoundEffectByID(ctx context.Context, id int64) (*soundmodel.SoundEffect, error) {
	if se, ok := bs.idToSoundEffect[id]; ok {
		return se, nil
	}
	return nil, he.HTTPCodedErrorf(404, "sound effect not found")
}

// FetchSoundEffectSlugs implements SoundStorage.
func (bs *BuiltinSoundStorage) FetchSoundEffectSlugs(ctx context.Context) ([]*soundmodel.SoundEffectSlug, error) {
	bi := builtins.SoundEffects()
	slugs := make([]*soundmodel.SoundEffectSlug, 0, len(bi))

	for _, se := range builtins.SoundEffects() {
		slugs = append(slugs, &soundmodel.SoundEffectSlug{
			ID:          se.ID,
			Name:        se.Name,
			Description: se.Description,
			Path:        se.Path,
		})
	}
	return slugs, nil
}

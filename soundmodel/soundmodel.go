package soundmodel

// View object for picking sound effects.
type SoundEffectSlug struct {
	ID          int64
	Name        string
	Description string
	Path        string
}

// Imaginary database object which would also contain the sound blob.
type SoundEffect struct {
	ID          int64
	Name        string
	Description string
	Path        string
}

func (se *SoundEffect) Clone() *SoundEffect {
	return &SoundEffect{
		ID:          se.ID,
		Name:        se.Name,
		Description: se.Description,
		Path:        se.Path,
	}
}

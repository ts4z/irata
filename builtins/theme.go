package builtins

// Theme defines the visual styling parameters for a theme.
type Theme struct {
	Name            string
	MonoFont        string  // Monospace font filename (e.g., "PressStart2P-vaV7.ttf")
	SansFont        string  // Sans-serif font filename (e.g., "RedHatDisplay-VariableFont_wght.ttf")
	LineHeight      string  // CSS line-height value (e.g., "1.7" or "1")
	BodyFontWeight  string  // CSS font-weight for body (e.g., "normal" or "800")
	FontScaleFactor float64 // Fudge factor, essentially relative to PressStart2P font, to adjust font sizes.
}

// ThemeStorage holds all available themes.
type ThemeStorage struct {
	themes map[string]*Theme
}

// NewThemeStorage creates a new ThemeStorage with predefined themes.
func NewThemeStorage() *ThemeStorage {
	return &ThemeStorage{
		themes: map[string]*Theme{
			"irata": {
				Name:            "irata",
				MonoFont:        "PressStart2P-vaV7.ttf",
				SansFont:        "PressStart2P-vaV7.ttf",
				LineHeight:      "1.7",
				BodyFontWeight:  "normal",
				FontScaleFactor: 1.0,
			},
			"gambler": {
				Name:            "gambler",
				MonoFont:        "RedHatMono-VariableFont_wght.ttf",
				SansFont:        "RedHatDisplay-VariableFont_wght.ttf",
				LineHeight:      "1.1",
				BodyFontWeight:  "800",
				FontScaleFactor: 1.4,
			},
		},
	}
}

// GetTheme returns the theme with the given name, or nil if not found.
//
// TODO: Rename to FetchThemeByID?
func (ts *ThemeStorage) GetTheme(name string) *Theme {
	return ts.themes[name]
}

// ListThemes returns all available theme names.
//
// TODO: Rename to FetchThemeSlugs.
func (ts *ThemeStorage) ListThemes() []string {
	names := make([]string, 0, len(ts.themes))
	for name := range ts.themes {
		names = append(names, name)
	}
	return names
}

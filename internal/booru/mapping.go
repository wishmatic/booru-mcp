package booru

// Danbooru numbers categories general 0, artist 1, copyright 3, character 4, and meta 5; 2 is skipped.
func tagCategory(code int) TagCategory {
	switch code {
	case 1:
		return CategoryArtist
	case 3:
		return CategoryCopyright
	case 4:
		return CategoryCharacter
	case 5:
		return CategoryMeta
	default:
		return CategoryGeneral
	}
}

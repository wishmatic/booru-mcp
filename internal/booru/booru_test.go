package booru

import "testing"

func TestNormalizeTag(t *testing.T) {
	tests := map[string]string{
		"Blue Eyes":     "blue_eyes",
		"  BLUE  EYES ": "blue_eyes",
		"blue_eyes":     "blue_eyes",
		"":              "",
		"One":           "one",
	}

	for in, want := range tests {
		if got := NormalizeTag(in); got != want {
			t.Errorf("NormalizeTag(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestTagCategoryString(t *testing.T) {
	for _, category := range []TagCategory{CategoryGeneral, CategoryArtist, CategoryCopyright, CategoryCharacter, CategoryMeta} {
		if category.String() == "" {
			t.Errorf("TagCategory(%q).String() is empty", string(category))
		}
	}
}

package booru

type TagQuery struct {
	Search     string
	Categories []TagCategory
	Offset     int
	Limit      int
}

// TagPage is one window of the count-ordered tag matches. More reports whether at least one match exists past the
// window.
type TagPage struct {
	Tags []Tag
	More bool
}

type Implication struct {
	Antecedent string
	Consequent string
}

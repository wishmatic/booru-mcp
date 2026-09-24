package booru

type TagQuery struct {
	Search     string
	Categories []TagCategory
	Offset     int
	Limit      int
	MinCount   int
}

type TagPage struct {
	Tags []Tag
	More bool

	// Withheld counts matches the read pages held below the min-count floor. It is a lower bound: paging stops as soon
	// as the count-ordered listing drops to the floor, so further matches are known to exist but are not counted.
	Withheld int
	// WithheldBest is the work count of the highest-ranked withheld match, or zero when none was withheld.
	WithheldBest int
}

type Implication struct {
	Antecedent string
	Consequent string
}

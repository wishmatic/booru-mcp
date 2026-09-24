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

	// Matched reports that the search matches at least one tag, whether or not the floor or the requested offset
	// returned any. It is what separates "the offset ran past the end" from "nothing matches this search".
	Matched bool

	// Withheld counts matches the read pages held below the min-count floor. It is a lower bound: paging stops once
	// the listing drops below the floor, so matches past that page are known to exist but are not counted.
	Withheld int
	// WithheldBest is the work count of the highest-ranked withheld match, or zero when none was withheld.
	WithheldBest int
}

type Implication struct {
	Antecedent string
	Consequent string
}

type Alias struct {
	Antecedent string
	Consequent string
}

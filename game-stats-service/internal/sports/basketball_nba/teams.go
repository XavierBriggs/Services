package basketball_nba

// NBA team abbreviation mappings
var nbaTeamAbbreviations = map[string]string{
	"Atlanta Hawks":          "ATL",
	"Boston Celtics":         "BOS",
	"Brooklyn Nets":          "BKN",
	"Charlotte Hornets":      "CHA",
	"Chicago Bulls":          "CHI",
	"Cleveland Cavaliers":    "CLE",
	"Dallas Mavericks":       "DAL",
	"Denver Nuggets":         "DEN",
	"Detroit Pistons":        "DET",
	"Golden State Warriors":  "GSW",
	"Houston Rockets":        "HOU",
	"Indiana Pacers":         "IND",
	"Los Angeles Clippers":   "LAC",
	"Los Angeles Lakers":     "LAL",
	"Memphis Grizzlies":      "MEM",
	"Miami Heat":             "MIA",
	"Milwaukee Bucks":        "MIL",
	"Minnesota Timberwolves": "MIN",
	"New Orleans Pelicans":   "NOP",
	"New York Knicks":        "NYK",
	"Oklahoma City Thunder":  "OKC",
	"Orlando Magic":          "ORL",
	"Philadelphia 76ers":     "PHI",
	"Phoenix Suns":           "PHX",
	"Portland Trail Blazers": "POR",
	"Sacramento Kings":       "SAC",
	"San Antonio Spurs":      "SAS",
	"Toronto Raptors":        "TOR",
	"Utah Jazz":              "UTA",
	"Washington Wizards":     "WAS",
}

// Reverse mapping for lookups
var nbaAbbreviationToName = map[string]string{}

func init() {
	// Build reverse mapping
	for name, abbr := range nbaTeamAbbreviations {
		nbaAbbreviationToName[abbr] = name
	}
}

// GetTeamAbbreviation returns the abbreviation for a full team name
func GetTeamAbbreviation(fullName string) string {
	if abbr, ok := nbaTeamAbbreviations[fullName]; ok {
		return abbr
	}
	return fullName // Return original if not found
}

// GetTeamName returns the full name for an abbreviation
func GetTeamName(abbr string) string {
	if name, ok := nbaAbbreviationToName[abbr]; ok {
		return name
	}
	return abbr // Return original if not found
}





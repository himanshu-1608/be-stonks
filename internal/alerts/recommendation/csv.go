package recommendation

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// Recommendation is one parsed row used to build alerts.
type Recommendation struct {
	Symbol  string // exchange suffix stripped (e.g. "TARIL")
	Target1 float64
	Target2 float64
	Line    int // 1-based source line, for reporting
}

// Load parses the recommendation CSV. It returns valid recommendations and a
// list of human-readable skip reasons for malformed rows. A malformed row never
// aborts the parse; only an unreadable file returns an error.
//
// Expected columns: Stock Code/Name, Recommendation Date, Target 1, Target 2, Buy Price.
func Load(path string) ([]Recommendation, []string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.FieldsPerRecord = -1 // tolerate ragged rows; we validate per row

	var recs []Recommendation
	var skipped []string
	line := 0
	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, err
		}
		line++
		if line == 1 {
			continue // header
		}
		if len(row) < 4 {
			skipped = append(skipped, fmt.Sprintf("line %d: too few columns", line))
			continue
		}
		sym := strings.TrimSuffix(strings.ToUpper(strings.TrimSpace(row[0])), ".NS")
		t1, err1 := strconv.ParseFloat(strings.TrimSpace(row[2]), 64)
		t2, err2 := strconv.ParseFloat(strings.TrimSpace(row[3]), 64)
		if sym == "" || err1 != nil || err2 != nil {
			skipped = append(skipped, fmt.Sprintf("line %d: bad symbol or targets", line))
			continue
		}
		recs = append(recs, Recommendation{Symbol: sym, Target1: t1, Target2: t2, Line: line})
	}
	return recs, skipped, nil
}

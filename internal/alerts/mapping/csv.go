package mapping

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strconv"
)

// Entry is one row in the dedup/log CSV.
type Entry struct {
	Provider  string
	AlertName string
	Symbol    string
	Exchange  string
	Tier      string
	Value     float64
	CreatedAt string
}

var header = []string{"provider", "alert_name", "symbol", "exchange", "tier", "value", "created_at"}

// Load reads all entries. A missing file is not an error (returns nil, nil).
func Load(path string) ([]Entry, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	rows, err := r.ReadAll()
	if err != nil {
		return nil, err
	}

	var out []Entry
	for i, row := range rows {
		if i == 0 || len(row) < 7 {
			continue // header or malformed
		}
		v, _ := strconv.ParseFloat(row[5], 64)
		out = append(out, Entry{
			Provider:  row[0],
			AlertName: row[1],
			Symbol:    row[2],
			Exchange:  row[3],
			Tier:      row[4],
			Value:     v,
			CreatedAt: row[6],
		})
	}
	return out, nil
}

// Append adds one entry, creating the file (with header) if needed.
func Append(path string, e Entry) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	_, statErr := os.Stat(path)
	newFile := os.IsNotExist(statErr)

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	if newFile {
		if err := w.Write(header); err != nil {
			return err
		}
	}
	return w.Write([]string{
		e.Provider, e.AlertName, e.Symbol, e.Exchange, e.Tier,
		strconv.FormatFloat(e.Value, 'f', -1, 64), e.CreatedAt,
	})
}

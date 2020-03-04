package log

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// To support ad-hoc parsing in a better way we want to make order of fields in
// output JSON documents determistics. Additionally, this will make them more
// human-readable when values from multiple messages are lined up to each
// other.

func marshalOrderedJSON(output *strings.Builder, m map[string]interface{}) error {
	order := make([]string, 0, len(m))
	for k := range m {
		order = append(order, k)
	}
	sort.Strings(order)

	output.WriteRune('{')
	for i, key := range order {
		if i != 0 {
			output.WriteRune(',')
		}

		jsonKey, err := json.Marshal(key)
		if err != nil {
			return err
		}

		output.Write(jsonKey)
		output.WriteString(":")

		val := m[key]
		switch casted := val.(type) {
		case time.Time:
			val = casted.Format("2006-01-02T15:04:05.000")
		case time.Duration:
			val = casted.String()
		case LogFormatter:
			val = casted.FormatLog()
		case fmt.Stringer:
			val = casted.String()
		case error:
			val = casted.Error()
		}

		jsonValue, err := json.Marshal(val)
		if err != nil {
			return err
		}
		output.Write(jsonValue)
	}
	output.WriteRune('}')

	return nil
}

package log

import (
	"encoding/json"
	"sort"
	"strings"
)

// To support ad-hoc parsing in a better way we want to make order of fields in
// output JSON documents determistics. Additionally, this will make them more
// human-readable when values from multiple messages are lined up to each
// other.

func marshalOrderedJSON(output *strings.Builder, m map[string]interface{}) error {
	// TODO: Consider making maps used for error tracing and logging ordered in
	// the first place to avoid sorting overhead.
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

		jsonValue, err := json.Marshal(m[string(key)])
		if err != nil {
			return err
		}
		output.Write(jsonValue)
	}
	output.WriteRune('}')

	return nil
}

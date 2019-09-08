package dispatcher

import (
	"encoding/hex"
	"math/rand"
)

// GenerateMsgID generates a string usable as MsgID field in module.MsgMeta.
//
// TODO: Find a better place for this function. 'dispatcher' package seems
// irrelevant.
func GenerateMsgID() (string, error) {
	rawID := make([]byte, 32)
	_, err := rand.Read(rawID)
	return hex.EncodeToString(rawID), err
}

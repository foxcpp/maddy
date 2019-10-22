package msgpipeline

import (
	"crypto/rand"
	"encoding/hex"
)

// GenerateMsgID generates a string usable as MsgID field in module.MsgMeta.
//
// TODO: Find a better place for this function. 'msgpipeline' package seems
// irrelevant.
func GenerateMsgID() (string, error) {
	rawID := make([]byte, 32)
	_, err := rand.Read(rawID)
	return hex.EncodeToString(rawID), err
}

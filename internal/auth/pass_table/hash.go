/*
Maddy Mail Server - Composable all-in-one email server.
Copyright © 2019-2020 Max Mazurov <fox.cpp@disroot.org>, Maddy Mail Server contributors

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

package pass_table

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"io"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/bcrypt"
)

const (
	HashSHA256 = "sha256"
	HashBcrypt = "bcrypt"
	HashArgon2 = "argon2"

	DefaultHash = HashBcrypt

	Argon2Salt = 16
	Argon2Size = 64
)

type (
	// HashOpts is the structure that holds additional parameters for used hash
	// functions. They are used for new passwords.
	//
	// These parameters should be stored together with the hashed password
	// so it can be verified independently of the used HashOpts.
	HashOpts struct {
		// Bcrypt cost value to use. Should be at least 10.
		BcryptCost int

		Argon2Time    uint32
		Argon2Memory  uint32
		Argon2Threads uint8
	}

	FuncHashCompute func(opts HashOpts, pass string) (string, error)
	FuncHashVerify  func(pass, hashSalt string) error
)

var (
	HashCompute = map[string]FuncHashCompute{
		HashBcrypt: computeBcrypt,
		HashArgon2: computeArgon2,
	}
	HashVerify = map[string]FuncHashVerify{
		HashBcrypt: verifyBcrypt,
		HashArgon2: verifyArgon2,
	}

	Hashes = []string{HashSHA256, HashBcrypt, HashArgon2}
)

func computeArgon2(opts HashOpts, pass string) (string, error) {
	salt := make([]byte, Argon2Salt)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return "", fmt.Errorf("pass_table: failed to generate salt: %w", err)
	}

	hash := argon2.IDKey([]byte(pass), salt, opts.Argon2Time, opts.Argon2Memory, opts.Argon2Threads, Argon2Size)
	var out strings.Builder
	out.WriteString(strconv.FormatUint(uint64(opts.Argon2Time), 10))
	out.WriteRune(':')
	out.WriteString(strconv.FormatUint(uint64(opts.Argon2Memory), 10))
	out.WriteRune(':')
	out.WriteString(strconv.FormatUint(uint64(opts.Argon2Threads), 10))
	out.WriteRune(':')
	out.WriteString(base64.StdEncoding.EncodeToString(salt))
	out.WriteRune(':')
	out.WriteString(base64.StdEncoding.EncodeToString(hash))
	return out.String(), nil
}

func verifyArgon2(pass, hashSalt string) error {
	parts := strings.SplitN(hashSalt, ":", 5)

	time, err := strconv.ParseUint(parts[0], 10, 32)
	if err != nil {
		return fmt.Errorf("pass_table: malformed hash string: %w", err)
	}
	memory, err := strconv.ParseUint(parts[1], 10, 32)
	if err != nil {
		return fmt.Errorf("pass_table: malformed hash string: %w", err)
	}
	threads, err := strconv.ParseUint(parts[2], 10, 8)
	if err != nil {
		return fmt.Errorf("pass_table: malformed hash string: %w", err)
	}
	salt, err := base64.StdEncoding.DecodeString(parts[3])
	if err != nil {
		return fmt.Errorf("pass_table: malformed hash string: %w", err)
	}
	hash, err := base64.StdEncoding.DecodeString(parts[4])
	if err != nil {
		return fmt.Errorf("pass_table: malformed hash string: %w", err)
	}

	passHash := argon2.IDKey([]byte(pass), salt, uint32(time), uint32(memory), uint8(threads), Argon2Size)
	if subtle.ConstantTimeCompare(passHash, hash) != 1 {
		return fmt.Errorf("pass_table: hash mismatch")
	}
	return nil
}

func computeSHA256(_ HashOpts, pass string) (string, error) {
	salt := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return "", fmt.Errorf("pass_table: failed to generate salt: %w", err)
	}

	hashInput := salt
	hashInput = append(hashInput, []byte(pass)...)
	sum := sha256.Sum256(hashInput)
	return base64.StdEncoding.EncodeToString(salt) + ":" + base64.StdEncoding.EncodeToString(sum[:]), nil
}

func verifySHA256(pass, hashSalt string) error {
	parts := strings.Split(hashSalt, ":")
	if len(parts) != 2 {
		return fmt.Errorf("pass_table: malformed hash string, no salt")
	}
	salt, err := base64.StdEncoding.DecodeString(parts[0])
	if err != nil {
		return fmt.Errorf("pass_table: malformed hash string, cannot decode pass: %w", err)
	}
	hash, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return fmt.Errorf("pass_table: malformed hash string, cannot decode pass: %w", err)
	}

	hashInput := salt
	hashInput = append(hashInput, []byte(pass)...)
	sum := sha256.Sum256(hashInput)

	if subtle.ConstantTimeCompare(sum[:], hash) != 1 {
		return fmt.Errorf("pass_table: hash mismatch")
	}
	return nil
}

func computeBcrypt(opts HashOpts, pass string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(pass), opts.BcryptCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func verifyBcrypt(pass, hashSalt string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashSalt), []byte(pass))
}

func addSHA256() {
	HashCompute[HashSHA256] = computeSHA256
	HashVerify[HashSHA256] = verifySHA256
}

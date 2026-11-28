// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 madarlevi

package auth

import (
	"net/mail"
	"strings"
	"unicode/utf8"

	"github.com/madarlevi/kolikaja/apps/api/internal/platform/validate"
)

// Password rules (contract NewPassword): 12–128 code points, not a common password,
// not the e-mail address.
const (
	PasswordMinLength = 12
	PasswordMaxLength = 128
)

// commonPasswords is a small list of frequently used passwords of at least 12 characters.
// It is a minimum, not a breach database.
var commonPasswords = map[string]bool{
	"123456789012": true, "1234567890123": true, "12345678901234": true, "123456123456": true,
	"password1234": true, "password12345": true, "password123456": true, "passwordpassword": true,
	"qwertyuiop12": true, "qwertyuiop123": true, "qwertyuiopasdf": true, "1q2w3e4r5t6y": true,
	"iloveyou1234": true, "abc123abc123": true, "aaaaaaaaaaaa": true, "111111111111": true,
	"000000000000": true, "letmein12345": true, "welcome12345": true, "adminadmin12": true,
	"administrator": true, "jelszo123456": true, "jelszojelszo": true, "kolikaja1234": true,
	"kolikajakolikaja": true, "qwertzuiop12": true, "asdfghjkl123": true, "zxcvbnm12345": true,
}

func validatePassword(v *validate.Collector, field, password, email string) {
	n := utf8.RuneCountInString(password)
	switch {
	case !utf8.ValidString(password):
		v.Add(field, "invalid_format")
	case n < PasswordMinLength:
		v.Add(field, "too_short")
	case n > PasswordMaxLength:
		v.Add(field, "too_long")
	case commonPasswords[strings.ToLower(password)] || (email != "" && strings.EqualFold(password, email)):
		v.Add(field, "too_common")
	}
}

// normalizeEmail trims the address and checks its format and length. The address is
// stored as typed; uniqueness is case-insensitive (lower(email) index).
func normalizeEmail(v *validate.Collector, field, raw string) string {
	email := strings.TrimSpace(raw)
	if utf8.RuneCountInString(email) < 3 || len(email) > 254 {
		v.Add(field, "invalid_format")
		return email
	}
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email || !strings.Contains(email, "@") {
		v.Add(field, "invalid_format")
	}
	return email
}

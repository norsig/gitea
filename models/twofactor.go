// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"crypto/md5"
	"crypto/subtle"
	"encoding/base64"

	"github.com/Unknwon/com"
	"github.com/pquerna/otp/totp"

	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

// TwoFactor represents a two-factor authentication token.
type TwoFactor struct {
	ID           int64 `xorm:"pk autoincr"`
	UID          int64 `xorm:"UNIQUE"`
	Secret       string
	ScratchToken string
	CreatedUnix  util.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix  util.TimeStamp `xorm:"INDEX updated"`
}

// GenerateScratchToken recreates the scratch token the user is using.
func (t *TwoFactor) GenerateScratchToken() error {
	token, err := base.GetRandomString(8)
	if err != nil {
		return err
	}
	t.ScratchToken = token
	return nil
}

// VerifyScratchToken verifies if the specified scratch token is valid.
func (t *TwoFactor) VerifyScratchToken(token string) bool {
	if len(token) == 0 {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(token), []byte(t.ScratchToken)) == 1
}

func (t *TwoFactor) getEncryptionKey() []byte {
	k := md5.Sum([]byte(setting.SecretKey))
	return k[:]
}

// SetSecret sets the 2FA secret.
func (t *TwoFactor) SetSecret(secret string) error {
	secretBytes, err := com.AESGCMEncrypt(t.getEncryptionKey(), []byte(secret))
	if err != nil {
		return err
	}
	t.Secret = base64.StdEncoding.EncodeToString(secretBytes)
	return nil
}

// ValidateTOTP validates the provided passcode.
func (t *TwoFactor) ValidateTOTP(passcode string) (bool, error) {
	decodedStoredSecret, err := base64.StdEncoding.DecodeString(t.Secret)
	if err != nil {
		return false, err
	}
	secret, err := com.AESGCMDecrypt(t.getEncryptionKey(), decodedStoredSecret)
	if err != nil {
		return false, err
	}
	secretStr := string(secret)
	return totp.Validate(passcode, secretStr), nil
}

// NewTwoFactor creates a new two-factor authentication token.
func NewTwoFactor(t *TwoFactor) error {
	err := t.GenerateScratchToken()
	if err != nil {
		return err
	}
	_, err = x.Insert(t)
	return err
}

// UpdateTwoFactor updates a two-factor authentication token.
func UpdateTwoFactor(t *TwoFactor) error {
	_, err := x.ID(t.ID).AllCols().Update(t)
	return err
}

// GetTwoFactorByUID returns the two-factor authentication token associated with
// the user, if any.
func GetTwoFactorByUID(uid int64) (*TwoFactor, error) {
	twofa := &TwoFactor{UID: uid}
	has, err := x.Get(twofa)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrTwoFactorNotEnrolled{uid}
	}
	return twofa, nil
}

// DeleteTwoFactorByID deletes two-factor authentication token by given ID.
func DeleteTwoFactorByID(id, userID int64) error {
	cnt, err := x.ID(id).Delete(&TwoFactor{
		UID: userID,
	})
	if err != nil {
		return err
	} else if cnt != 1 {
		return ErrTwoFactorNotEnrolled{userID}
	}
	return nil
}

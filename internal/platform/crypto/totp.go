package crypto

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const totpPeriodSeconds = 30

func VerifyTOTP(
	encodedSecret string,
	code string,
	now time.Time,
	lastUsedStep int64,
) (int64, bool) {
	if len(code) != 6 || now.Unix() < 0 {
		return 0, false
	}
	if _, err := strconv.ParseUint(code, 10, 32); err != nil {
		return 0, false
	}
	secret, err := base32.StdEncoding.WithPadding(base32.NoPadding).
		DecodeString(strings.ToUpper(encodedSecret))
	if err != nil || len(secret) < 16 {
		return 0, false
	}
	defer clear(secret)

	currentStep := now.Unix() / totpPeriodSeconds
	var matchedStep int64 = -1
	for _, candidate := range []int64{currentStep - 1, currentStep, currentStep + 1} {
		if candidate <= lastUsedStep || candidate < 0 {
			continue
		}
		expected := totpCode(secret, uint64(candidate))
		if subtle.ConstantTimeCompare([]byte(expected), []byte(code)) == 1 &&
			candidate > matchedStep {
			matchedStep = candidate
		}
	}
	return matchedStep, matchedStep >= 0
}

func totpCode(secret []byte, step uint64) string {
	var counter [8]byte
	binary.BigEndian.PutUint64(counter[:], step)
	mac := hmac.New(sha1.New, secret)
	_, _ = mac.Write(counter[:])
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	value := binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff
	return fmt.Sprintf("%06d", value%1_000_000)
}

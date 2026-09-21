package domain

import (
	"encoding/base32"
	"net/url"
	"strconv"
	"strings"
	"unicode"

	"github.com/google/uuid"
)

// GenerateReference builds the alphanumeric payment transfer reference for an order.
// Format: "FLU" + base32(order_id)[:10] in uppercase.
func GenerateReference(orderID uuid.UUID) string {
	encoded := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(orderID[:])
	if len(encoded) > 10 {
		encoded = encoded[:10]
	}
	return "FLU" + strings.ToUpper(encoded)
}

// BuildVietQRURL constructs the VietQR image URL as documented by SePay.
// Format: https://vietqr.app/img?acc=<acc>&bank=<bank>&amount=<vnd>&des=<ref>&template=compact
func BuildVietQRURL(bankCode, accountNumber string, amountVND int64, reference string) string {
	v := url.Values{}
	v.Set("acc", accountNumber)
	v.Set("bank", bankCode)
	v.Set("amount", strconv.FormatInt(amountVND, 10))
	v.Set("des", reference)
	v.Set("template", "compact")
	return "https://vietqr.app/img?" + v.Encode()
}

// StripNonAlphanumeric removes all characters that are not ASCII letters or digits.
func StripNonAlphanumeric(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ContentMatchesReference returns true if the transfer content contains the reference,
// comparing case-insensitively after stripping non-alphanumeric characters.
func ContentMatchesReference(content, reference string) bool {
	cleanContent := strings.ToUpper(StripNonAlphanumeric(content))
	cleanRef := strings.ToUpper(StripNonAlphanumeric(reference))
	if cleanRef == "" {
		return false
	}
	return strings.Contains(cleanContent, cleanRef)
}

// Package phlogmask masks sensitive values in the copies of payloads, headers,
// URLs, errors and audit-trail or trace payloads that services write to their
// logs and audit stores. It never touches the bytes sent over the wire:
// callers pass it a copy for logging only.
//
// Masking is by key name, not by value pattern, so only fields known to hold
// a secret or customer data change. IDs, references, amounts, codes, statuses
// and lengths are left as they are, and the same input always masks to the
// same output, so a masked line still explains the call and still correlates
// with other lines (FT-235 D7: masking must not get in the way of debugging).
//
// It is used by the FT services ftsnapinterface-manager, fundtransfer-module
// and fundtransferh2h-manager; the key lists follow SNAP and FT payloads.
//
// The package reads no environment. Call Resolve(appEnv, configured) and then
// SetEnabled once at boot. The zero value masks, so a process that never
// calls SetEnabled masks.
package phlogmask

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"unicode/utf8"
)

// disabled is set once at boot by main, from Resolve. Its zero value (false)
// means masking is on, so a test or a caller that never calls SetEnabled
// masks. The env default (LOG_MASK_SENSITIVE_DATA unset) is off; main applies
// it explicitly.
var disabled atomic.Bool

// SetEnabled turns masking on (true) or off (false). With masking off, JSON,
// Headers, URL and Err return what they were given, so the dumps are exactly
// what they were before masking existed. Call it once at boot.
func SetEnabled(on bool) { disabled.Store(!on) }

// Enabled reports whether masking is on.
func Enabled() bool { return !disabled.Load() }

// Resolve decides whether masking is on from APP_ENV and the configured
// LOG_MASK_SENSITIVE_DATA value. Production always masks, whatever is
// configured; every other environment follows the configured value. forced
// reports that production turned on a masking the configuration left off,
// so the caller can warn about the misconfiguration.
func Resolve(appEnv string, configured bool) (on, forced bool) {
	if strings.EqualFold(strings.TrimSpace(appEnv), "production") {
		return true, !configured
	}
	return configured, false
}

// kind is how a value is masked.
type kind int

const (
	kindNone    kind = iota
	kindSecret       // whole value replaced: tokens, signatures, keys, OTP
	kindAccount      // all but the last 4 characters starred
	kindName         // first letter of each word kept
	kindEmail        // first letter of the local part and the domain kept
	kindPhone        // all but the last 3 characters starred
	kindText         // free text: replaced by its length
)

const redacted = "[REDACTED]"

// exact maps a normalized key (lower case, no '_' or '-') to its kind.
var exact = map[string]kind{
	// secrets
	"authorization": kindSecret, "cookie": kindSecret, "setcookie": kindSecret,
	"otp": kindSecret, "pin": kindSecret, "authcode": kindSecret, "passapp": kindSecret,
	// account-like numbers
	"accountno": kindAccount, "accountnumber": kindAccount,
	"beneficiaryaccountno": kindAccount, "beneficiaryaccountnumber": kindAccount,
	"sourceaccount": kindAccount, "sourceaccountno": kindAccount, "sourceaccountnumber": kindAccount,
	"hashedsourceaccountno": kindAccount, "destinationaccountnumber": kindAccount,
	"bankaccountnumber": kindAccount, "collectionaccountno": kindAccount, "collectionaccountnumber": kindAccount,
	"payeraccountno": kindAccount, "payerid": kindAccount,
	"virtualaccountno": kindAccount, "vanumber": kindAccount,
	"customerno": kindAccount, "originatorcustomerno": kindAccount,
	// names
	"name": kindName, "accountname": kindName, "accountholder": kindName, "accountholdername": kindName,
	"beneficiaryaccountname": kindName, "beneficiaryname": kindName, "recipientname": kindName,
	"originatorcustomername": kindName, "customername": kindName, "virtualaccountname": kindName,
	"sendername": kindName, "payer": kindName, "validatedaccountname": kindName,
	"verifiedaccountname": kindName, "destinationaccountholdername": kindName,
	"bankaccountholdername": kindName, "billname": kindName, "billshortname": kindName,
	// free text
	"remark": kindText, "description": kindText, "detailinfo": kindText, "freetexts": kindText,
	"kodepos": kindText,
}

// classify returns how the value under key is masked.
func classify(key string) kind {
	k := strings.NewReplacer("_", "", "-", "").Replace(strings.ToLower(key))
	if v, ok := exact[k]; ok {
		return v
	}
	switch {
	case k == "tokentype":
		return kindNone
	case strings.Contains(k, "token"), strings.Contains(k, "secret"), strings.Contains(k, "password"),
		strings.Contains(k, "signature"), strings.Contains(k, "apikey"), strings.Contains(k, "privatekey"):
		return kindSecret
	case strings.Contains(k, "email"):
		return kindEmail
	case strings.Contains(k, "phone"):
		return kindPhone
	case strings.Contains(k, "address"):
		return kindText
	}
	return kindNone
}

// Value masks v as the value of key. Keys that are not sensitive, and empty
// values, are returned unchanged, as is everything when masking is off.
func Value(key, v string) string {
	if !Enabled() {
		return v
	}
	return apply(classify(key), v)
}

func apply(k kind, v string) string {
	if v == "" || k == kindNone {
		return v
	}
	switch k {
	case kindSecret:
		return redacted
	case kindAccount:
		return keepLast(v, 4)
	case kindPhone:
		return keepLast(v, 3)
	case kindName:
		return initials(v)
	case kindEmail:
		at := strings.LastIndexByte(v, '@')
		if at <= 0 {
			return initials(v)
		}
		r, _ := utf8.DecodeRuneInString(v)
		return string(r) + "***" + v[at:]
	case kindText:
		return "[REDACTED len=" + itoa(utf8.RuneCountInString(v)) + "]"
	}
	return v
}

func keepLast(v string, n int) string {
	r := []rune(v)
	if len(r) <= n {
		return strings.Repeat("*", len(r))
	}
	return strings.Repeat("*", len(r)-n) + string(r[len(r)-n:])
}

func initials(v string) string {
	words := strings.Fields(v)
	for i, w := range words {
		r := []rune(w)
		words[i] = string(r[0]) + strings.Repeat("*", len(r)-1)
	}
	return strings.Join(words, " ")
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// Headers returns a copy of h with sensitive header values masked. An
// Authorization value keeps its scheme ("Bearer [REDACTED]"). X-CLIENT-KEY and
// X-PARTNER-ID are identifiers, not secrets, and are kept.
func Headers(h http.Header) http.Header {
	if h == nil || !Enabled() {
		return h
	}
	out := make(http.Header, len(h))
	for name, vals := range h {
		k := classify(name)
		cp := make([]string, len(vals))
		for i, v := range vals {
			switch {
			case k == kindNone:
				cp[i] = v
			case strings.EqualFold(name, "Authorization"):
				if scheme, _, ok := strings.Cut(v, " "); ok {
					cp[i] = scheme + " " + redacted
				} else {
					cp[i] = redacted
				}
			default:
				cp[i] = apply(k, v)
			}
		}
		out[name] = cp
	}
	return out
}

// accountDigits recognizes a whole account or VA number in a known VA route.
var accountDigits = regexp.MustCompile(`^\d{10,}$`)

// URL masks a password in the userinfo, the values of sensitive query keys,
// "key=value" path segments with a sensitive key, and numeric VA values in
// callback_virtual_accounts routes (bare or external_id=). Numeric IDs and
// references elsewhere stay readable. It works on the raw string so query
// order and escaping stay as sent; the scheme and host are never changed.
func URL(raw string) string {
	if !Enabled() {
		return raw
	}
	raw, fragment, hasFragment := strings.Cut(raw, "#")
	base, query, hasQuery := strings.Cut(raw, "?")
	if u, err := url.Parse(base); err == nil && u.User != nil {
		if _, ok := u.User.Password(); ok {
			base = u.Redacted()
		}
	}
	segs := strings.Split(base, "/")
	first := 0
	if strings.Contains(base, "://") {
		first = 3 // "https:", "", host
	}
	for i := first; i < len(segs); i++ {
		vaPath := i > first && segs[i-1] == "callback_virtual_accounts"
		if k, v, ok := strings.Cut(segs[i], "="); ok {
			maskKey := k
			if vaPath && k == "external_id" && accountDigits.MatchString(v) {
				maskKey = "virtualAccountNo"
			}
			if classify(maskKey) != kindNone {
				segs[i] = k + "=" + Value(maskKey, v)
			}
			continue
		}
		if vaPath && accountDigits.MatchString(segs[i]) {
			segs[i] = keepLast(segs[i], 4)
		}
	}
	out := strings.Join(segs, "/")
	if hasQuery {
		pairs := strings.Split(query, "&")
		for i, p := range pairs {
			if k, v, ok := strings.Cut(p, "="); ok {
				pairs[i] = k + "=" + Value(k, v)
			}
		}
		out += "?" + strings.Join(pairs, "&")
	}
	if hasFragment {
		out += "#" + fragment
	}
	return out
}

// JSON returns body with sensitive values masked. Valid JSON comes back
// compact, with key order and number literals kept, so logJSON can indent it
// as before. A value under a sensitive key that is an object or array has
// every scalar inside masked the same way, unless a nested key is itself
// sensitive. Anything that is not valid JSON goes through a "key":"value"
// regex instead, so a cut-off body is still masked and an HTML error page is
// returned unchanged.
func JSON(body []byte) []byte {
	if len(bytes.TrimSpace(body)) == 0 || !Enabled() {
		return body
	}
	if out, err := maskJSON(body); err == nil {
		return out
	}
	return maskLoose(body)
}

type frame struct {
	object    bool
	expectKey bool
	key       string // last key read in this object
	inherit   kind   // kind of the nearest sensitive ancestor key
	n         int    // items written, for commas
}

func maskJSON(body []byte) ([]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	enc.SetEscapeHTML(false)
	writeString := func(s string) {
		_ = enc.Encode(s)
		out.Truncate(out.Len() - 1) // Encode appends '\n'
	}
	var stack []*frame
	// current kind for the next value
	valueKind := func() kind {
		if len(stack) == 0 {
			return kindNone
		}
		top := stack[len(stack)-1]
		if top.object {
			if k := classify(top.key); k != kindNone {
				return k
			}
		}
		return top.inherit
	}
	sep := func() {
		if len(stack) == 0 {
			return
		}
		top := stack[len(stack)-1]
		if top.object && !top.expectKey {
			out.WriteByte(':')
			return
		}
		if top.n > 0 {
			out.WriteByte(',')
		}
		top.n++
	}
	afterValue := func() {
		if len(stack) > 0 && stack[len(stack)-1].object {
			stack[len(stack)-1].expectKey = true
		}
	}
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case json.Delim:
			switch t {
			case '{', '[':
				k := valueKind()
				sep()
				out.WriteByte(byte(t))
				stack = append(stack, &frame{object: t == '{', expectKey: t == '{', inherit: k})
			case '}', ']':
				stack = stack[:len(stack)-1]
				out.WriteByte(byte(t))
				afterValue()
			}
		case string:
			if len(stack) > 0 && stack[len(stack)-1].object && stack[len(stack)-1].expectKey {
				top := stack[len(stack)-1]
				sep()
				writeString(t)
				top.key = t
				top.expectKey = false
				continue
			}
			k := valueKind()
			sep()
			writeString(apply(k, t))
			afterValue()
		case json.Number:
			k := valueKind()
			sep()
			if k == kindNone {
				out.WriteString(t.String())
			} else {
				writeString(apply(k, t.String()))
			}
			afterValue()
		case bool:
			sep()
			if t {
				out.WriteString("true")
			} else {
				out.WriteString("false")
			}
			afterValue()
		case nil:
			sep()
			out.WriteString("null")
			afterValue()
		}
	}
	if len(stack) != 0 {
		return nil, io.ErrUnexpectedEOF
	}
	return out.Bytes(), nil
}

// loosePair matches "key": "value" and "key": number in text that is not
// valid JSON, including strings cut off mid-value or mid-escape at EOF.
var loosePair = regexp.MustCompile(`"([^"\\]{1,64})"(\s*:\s*)("(?:[^"\\]|\\.)*(?:"|\\?\z)|-?\d[\d.eE+-]*)`)

func maskLoose(body []byte) []byte {
	return loosePair.ReplaceAllFunc(body, func(m []byte) []byte {
		sub := loosePair.FindSubmatch(m)
		key, colon, val := string(sub[1]), string(sub[2]), string(sub[3])
		k := classify(key)
		if k == kindNone {
			return m
		}
		plain := strings.Trim(val, `"`)
		b, _ := json.Marshal(apply(k, plain))
		return []byte(`"` + key + `"` + colon + string(b))
	})
}

// Err renders err for a log line. A *url.Error, which net/http returns for
// transport failures, carries the full request URL; with masking on, only that
// URL is masked, in place, so wrapper text such as "http call exhausted
// retries:" and the transport cause stay (FT-236 D4). If a wrapper rewrote
// the url.Error text so it cannot be found, only the masked url.Error is
// returned, never the raw URL. Other errors are returned as err.Error(). A nil
// error renders "<nil>", as %v would.
func Err(err error) string {
	if err == nil {
		return "<nil>"
	}
	text := err.Error()
	var ue *url.Error
	if !Enabled() || !errors.As(err, &ue) {
		return text
	}
	masked := ue.Op + " " + strconv.Quote(URL(ue.URL)) + ": " + ue.Err.Error()
	if !strings.Contains(text, ue.Error()) {
		return masked
	}
	text = strings.ReplaceAll(text, ue.Error(), masked)
	if m := URL(ue.URL); m != ue.URL {
		text = strings.ReplaceAll(text, ue.URL, m)
	}
	return text
}

// Payload returns a masked copy of v for an audit-trail or trace payload
// (FT-236 F4, F7, F8). v is marshaled to JSON, masked by key as JSON does, and
// decoded again, so the audit store still gets a structured document. Numbers
// decode as json.Number, so IDs and amounts keep every digit. A string that
// holds a JSON object or array (a vendor request or reply kept as text) is
// masked inside and stays a string. With masking off, or when v cannot be
// marshaled, v itself is returned, so the record is exactly what it was.
func Payload(v any) any {
	if v == nil || !Enabled() {
		return v
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return v
	}
	dec := json.NewDecoder(bytes.NewReader(JSON(raw)))
	dec.UseNumber()
	var out any
	if err := dec.Decode(&out); err != nil {
		return v
	}
	return maskEmbedded(out)
}

// maskEmbedded masks JSON documents carried inside string values, in place.
func maskEmbedded(v any) any {
	switch t := v.(type) {
	case map[string]any:
		for k, item := range t {
			t[k] = maskEmbedded(item)
		}
	case []any:
		for i, item := range t {
			t[i] = maskEmbedded(item)
		}
	case string:
		s := strings.TrimSpace(t)
		if (strings.HasPrefix(s, "{") || strings.HasPrefix(s, "[")) && json.Valid([]byte(s)) {
			return string(JSON([]byte(s)))
		}
	}
	return v
}

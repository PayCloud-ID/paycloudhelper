package phlogmask

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

func TestValue(t *testing.T) {
	tests := []struct {
		key, in, want string
	}{
		{"beneficiaryAccountNo", "1234567890", "******7890"},
		{"account_number", "123", "***"},
		{"sourceAccount", "0011223344", "******3344"},
		{"virtualAccountNo", "8808123456789012", "************9012"},
		{"beneficiaryAccountName", "Budi Santoso", "B*** S******"},
		{"account_holder_name", "PT. Maju Jaya", "P** M*** J***"},
		{"beneficiaryEmail", "budi.s@example.com", "b***@example.com"},
		{"recieverPhone", "081234567890", "*********890"},
		{"remark", "gaji november budi", "[REDACTED len=18]"},
		{"beneficiaryAddress1", "Jl. Sudirman 1", "[REDACTED len=14]"},
		{"accessToken", "eyJhbGciOi", "[REDACTED]"},
		{"clientSecret", "s3cr3t", "[REDACTED]"},
		{"X-SIGNATURE", "abc==", "[REDACTED]"},
		{"otp", "123456", "[REDACTED]"},
		{"tokenType", "Bearer", "Bearer"},
		{"partnerReferenceNo", "h2h-transfer-1-detail-3", "h2h-transfer-1-detail-3"},
		{"customerReference", "REF-77", "REF-77"},
		{"responseCode", "2001800", "2001800"},
		{"bankCode", "014", "014"},
		{"beneficiaryAccountNo", "", ""},
	}
	for _, tt := range tests {
		if got := Value(tt.key, tt.in); got != tt.want {
			t.Errorf("Value(%q, %q) = %q, want %q", tt.key, tt.in, got, tt.want)
		}
	}
}

func TestJSON_MasksOnlySensitiveKeysAndKeepsOrderAndNumbers(t *testing.T) {
	in := `{"partnerReferenceNo":"h2h-1","beneficiaryAccountNo":"1234567890","beneficiaryAccountName":"Budi Santoso",` +
		`"amount":{"value":"10000.00","currency":"IDR"},"fee":1.50,"remark":"gaji","sourceAccountNo":9876543210,` +
		`"additionalInfo":{"deviceId":"d-1","beneficiaryAccountName":"Ani"},"ok":true,"x":null}`
	want := `{"partnerReferenceNo":"h2h-1","beneficiaryAccountNo":"******7890","beneficiaryAccountName":"B*** S******",` +
		`"amount":{"value":"10000.00","currency":"IDR"},"fee":1.50,"remark":"[REDACTED len=4]","sourceAccountNo":"******3210",` +
		`"additionalInfo":{"deviceId":"d-1","beneficiaryAccountName":"A**"},"ok":true,"x":null}`
	if got := string(JSON([]byte(in))); got != want {
		t.Errorf("JSON() =\n%s\nwant\n%s", got, want)
	}
}

func TestJSON_ContainerUnderSensitiveKeyIsMaskedThrough(t *testing.T) {
	in := `{"freeTexts":[{"english":"pay Budi","indonesia":"bayar Budi"}],"detailData":[{"remark":"TRF BUDI","amount":{"value":"5.00"},"detailInfo":{"note":"x"}}]}`
	want := `{"freeTexts":[{"english":"[REDACTED len=8]","indonesia":"[REDACTED len=10]"}],"detailData":[{"remark":"[REDACTED len=8]","amount":{"value":"5.00"},"detailInfo":{"note":"[REDACTED len=1]"}}]}`
	if got := string(JSON([]byte(in))); got != want {
		t.Errorf("JSON() =\n%s\nwant\n%s", got, want)
	}
}

func TestJSON_ArrayOfScalarsAndHTMLCharsStayReadable(t *testing.T) {
	in := `{"customerNo":["1234567890"],"note":"a<b & c>d","remark":"50% fee"}`
	want := `{"customerNo":["******7890"],"note":"a<b & c>d","remark":"[REDACTED len=7]"}`
	if got := string(JSON([]byte(in))); got != want {
		t.Errorf("JSON() = %s, want %s", got, want)
	}
}

func TestJSON_NotJSON(t *testing.T) {
	tests := []struct{ name, in, want string }{
		{"empty", "", ""},
		{"html error page unchanged", "<html>502 Bad Gateway</html>", "<html>502 Bad Gateway</html>"},
		{"cut-off JSON still masked", `{"partnerReferenceNo":"r-1","beneficiaryAccountNo":"1234567890","accessToken":"ey`,
			`{"partnerReferenceNo":"r-1","beneficiaryAccountNo":"******7890","accessToken":"[REDACTED]"`},
		{"cut-off JSON with complete secret", `{"accessToken":"eyJ.abc","expiresIn":"900"`, `{"accessToken":"[REDACTED]","expiresIn":"900"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := string(JSON([]byte(tt.in))); got != tt.want {
				t.Errorf("JSON(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestJSON_TruncatedSensitiveStringsKeepDebuggingFields(t *testing.T) {
	const prefix = `{"partnerReferenceNo":"h2h-transfer-1234567890-detail-3","amount":1.50,`
	tests := []struct{ name, tail, want string }{
		{"unfinished token", `"accessToken":"synthetic-secret-prefix`, `"accessToken":"[REDACTED]"`},
		{"unfinished signature", `"X-SIGNATURE":"synthetic-signature`, `"X-SIGNATURE":"[REDACTED]"`},
		{"unfinished escape", `"clientSecret":"synthetic-secret\`, `"clientSecret":"[REDACTED]"`},
		{"escaped quote", `"password":"synthetic-\"secret`, `"password":"[REDACTED]"`},
		{"unfinished unicode escape", `"ACCESS_TOKEN":"synthetic-\u00`, `"ACCESS_TOKEN":"[REDACTED]"`},
		{"unfinished reference unchanged", `"customerReference":"ref-1234567890`, `"customerReference":"ref-1234567890`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := prefix + tt.tail
			body := []byte(in)
			if got := string(JSON(body)); got != prefix+tt.want {
				t.Errorf("JSON() = %q, want %q", got, prefix+tt.want)
			}
			if string(body) != in {
				t.Error("JSON() changed the caller's body")
			}
		})
	}
}

func TestHeaders(t *testing.T) {
	in := http.Header{
		"Authorization":    {"Bearer eyJhbGciOi"},
		"X-Signature":      {"sig=="},
		"X-Client-Key":     {"client-1"},
		"X-Partner-Id":     {"partner-1"},
		"X-Timestamp":      {"2026-11-12T10:00:00+07:00"},
		"Content-Type":     {"application/json"},
		"X-Callback-Token": {"cb-secret"},
		"Set-Cookie":       {"sid=abc"},
	}
	got := Headers(in)
	want := map[string]string{
		"Authorization": "Bearer [REDACTED]", "X-Signature": "[REDACTED]", "X-Client-Key": "client-1",
		"X-Partner-Id": "partner-1", "X-Timestamp": "2026-11-12T10:00:00+07:00", "Content-Type": "application/json",
		"X-Callback-Token": "[REDACTED]", "Set-Cookie": "[REDACTED]",
	}
	for k, w := range want {
		if g := got.Get(k); g != w {
			t.Errorf("Headers()[%s] = %q, want %q", k, g, w)
		}
	}
	if in.Get("Authorization") != "Bearer eyJhbGciOi" {
		t.Error("Headers() changed the caller's header map; it must mask a copy")
	}
	if Headers(nil) != nil {
		t.Error("Headers(nil) != nil")
	}
	if g := Headers(http.Header{"Authorization": {"opaque"}}).Get("Authorization"); g != "[REDACTED]" {
		t.Errorf("schemeless Authorization = %q", g)
	}
}

func TestURL(t *testing.T) {
	tests := []struct{ in, want string }{
		{"https://vendor.example/transfer-intrabank", "https://vendor.example/transfer-intrabank"},
		{"https://api.xendit.co/callback_virtual_accounts?account_number=8808123456789012", "https://api.xendit.co/callback_virtual_accounts?account_number=************9012"},
		{"https://api.xendit.co/callback_virtual_accounts/external_id=8808123456789012/simulate_payment", "https://api.xendit.co/callback_virtual_accounts/external_id=************9012/simulate_payment"},
		{"https://api.xendit.co/callback_virtual_accounts/8808123456789012", "https://api.xendit.co/callback_virtual_accounts/************9012"},
		{"https://flip.example/get-disbursement?idempotency_key=h2h-transfer-1-detail-3", "https://flip.example/get-disbursement?idempotency_key=h2h-transfer-1-detail-3"},
		{"https://api.xendit.co/disbursements/h2h-transfer-1-detail-3", "https://api.xendit.co/disbursements/h2h-transfer-1-detail-3"},
		{"https://api.xendit.co/disbursements/h2h-transfer-1234567890-detail-3", "https://api.xendit.co/disbursements/h2h-transfer-1234567890-detail-3"},
		{"https://api.xendit.co/disbursements/1234567890?partnerReferenceNo=1234567890", "https://api.xendit.co/disbursements/1234567890?partnerReferenceNo=1234567890"},
		{"https://vendor.example/transfers/referenceNo=1234567890", "https://vendor.example/transfers/referenceNo=1234567890"},
		{"https://api.xendit.co/callback_virtual_accounts/external_id=h2h-transfer-1234567890-detail-3/simulate_payment", "https://api.xendit.co/callback_virtual_accounts/external_id=h2h-transfer-1234567890-detail-3/simulate_payment"},
		{"https://api.xendit.co/callback_virtual_accounts/8808123456789012?partnerReferenceNo=1234567890&amount=10000.00", "https://api.xendit.co/callback_virtual_accounts/************9012?partnerReferenceNo=1234567890&amount=10000.00"},
		{"https://api.xendit.co/callback_virtual_accounts/8808123456789012#debug", "https://api.xendit.co/callback_virtual_accounts/************9012#debug"},
		{"https://api.xendit.co/callback_virtual_accounts/external_id=8808123456789012/simulate_payment#debug", "https://api.xendit.co/callback_virtual_accounts/external_id=************9012/simulate_payment#debug"},
		{"https://api.xendit.co/callback_virtual_accounts?account_number=8808123456789012#debug", "https://api.xendit.co/callback_virtual_accounts?account_number=************9012#debug"},
		{"https://vendor.example/beneficiaryAccountNo=1234567890?customerReference=1234567890", "https://vendor.example/beneficiaryAccountNo=******7890?customerReference=1234567890"},
		{"https://user:pw@vendor.example/x", "https://user:xxxxx@vendor.example/x"},
	}
	for _, tt := range tests {
		if got := URL(tt.in); got != tt.want {
			t.Errorf("URL(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
	if strings.Contains(URL("https://a:secretpw@h/x"), "secretpw") {
		t.Error("password leaked")
	}
}

func TestErr(t *testing.T) {
	if got := Err(nil); got != "<nil>" {
		t.Errorf("Err(nil) = %q", got)
	}
	ue := &url.Error{Op: "Post", URL: "https://api.xendit.co/callback_virtual_accounts/8808123456789012", Err: errors.New("dial tcp: i/o timeout")}
	wrapped := fmt.Errorf("call: %w", ue)
	want := `call: Post "https://api.xendit.co/callback_virtual_accounts/************9012": dial tcp: i/o timeout`
	if got := Err(wrapped); got != want {
		t.Errorf("Err() = %q, want %q", got, want)
	}
	if got := Err(errors.New("plain")); got != "plain" {
		t.Errorf("Err(plain) = %q", got)
	}
}

func TestDisabledReturnsInputUnchanged(t *testing.T) {
	SetEnabled(false)
	t.Cleanup(func() { SetEnabled(true) })

	body := []byte(`{"beneficiaryAccountNo":"1234567890","accessToken":"ey"}`)
	if got := JSON(body); string(got) != string(body) {
		t.Errorf("JSON() = %s, want the input", got)
	}
	h := http.Header{"Authorization": {"Bearer ey"}}
	if got := Headers(h).Get("Authorization"); got != "Bearer ey" {
		t.Errorf("Headers() Authorization = %q, want the input", got)
	}
	raw := "https://u:pw@api.xendit.co/callback_virtual_accounts/8808123456789012?account_number=8808123456789012"
	if got := URL(raw); got != raw {
		t.Errorf("URL() = %q, want the input", got)
	}
	ue := &url.Error{Op: "Post", URL: raw, Err: errors.New("timeout")}
	if got := Err(ue); got != ue.Error() {
		t.Errorf("Err() = %q, want %q", got, ue.Error())
	}
	if got := Value("beneficiaryAccountNo", "1234567890"); got != "1234567890" {
		t.Errorf("Value() = %q, want the input", got)
	}
	if Enabled() {
		t.Error("Enabled() = true after SetEnabled(false)")
	}
}

func TestResolve(t *testing.T) {
	tests := []struct {
		appEnv             string
		configured         bool
		wantOn, wantForced bool
	}{
		{appEnv: "develop", configured: false, wantOn: false, wantForced: false},
		{appEnv: "develop", configured: true, wantOn: true, wantForced: false},
		{appEnv: "staging", configured: false, wantOn: false, wantForced: false},
		{appEnv: "staging", configured: true, wantOn: true, wantForced: false},
		{appEnv: "production", configured: true, wantOn: true, wantForced: false},
		{appEnv: "production", configured: false, wantOn: true, wantForced: true},
		{appEnv: " Production ", configured: false, wantOn: true, wantForced: true},
		{appEnv: "", configured: false, wantOn: false, wantForced: false},
	}
	for _, tt := range tests {
		on, forced := Resolve(tt.appEnv, tt.configured)
		if on != tt.wantOn || forced != tt.wantForced {
			t.Errorf("Resolve(%q, %v) = (%v, %v), want (%v, %v)", tt.appEnv, tt.configured, on, forced, tt.wantOn, tt.wantForced)
		}
	}
}

func TestErrKeepsWrapperText(t *testing.T) {
	ue := &url.Error{Op: "Get", URL: "https://api.xendit.co/callback_virtual_accounts?account_number=8808123456789012",
		Err: errors.New("read: connection reset by peer")}
	err := fmt.Errorf("http call exhausted retries: %w", ue)
	want := `http call exhausted retries: Get "https://api.xendit.co/callback_virtual_accounts?account_number=************9012": read: connection reset by peer`
	if got := Err(err); got != want {
		t.Errorf("Err() = %q, want %q", got, want)
	}
	SetEnabled(false)
	t.Cleanup(func() { SetEnabled(true) })
	if got := Err(err); got != err.Error() {
		t.Errorf("masking off: Err() = %q, want %q", got, err.Error())
	}
}

type rewrite struct{ ue *url.Error }

func (r rewrite) Error() string { return "custom text " + r.ue.URL }
func (r rewrite) Unwrap() error { return r.ue }

func TestErrUnknownWrapperNeverLeaks(t *testing.T) {
	ue := &url.Error{Op: "Get", URL: "https://api.xendit.co/callback_virtual_accounts/8808123456789012", Err: errors.New("EOF")}
	if got := Err(rewrite{ue}); strings.Contains(got, "8808123456789012") {
		t.Fatalf("Err() = %q leaks the VA number", got)
	}
}

func TestPayload_MasksStructsMapsAndJSONStrings(t *testing.T) {
	SetEnabled(true)
	type bene struct {
		AccountNo   string  `json:"beneficiaryAccountNo"`
		AccountName string  `json:"beneficiaryAccountName"`
		Email       string  `json:"beneficiaryEmail"`
		Amount      string  `json:"amount"`
		Ref         string  `json:"partnerReferenceNo"`
		DetailID    int64   `json:"detail_id"`
		Fee         float64 `json:"fee"`
	}
	in := map[string]any{
		"request":         bene{"9988776655", "Siti Erlina", "siti@example.com", "150000.00", "TRF20261113000468", 9007199254740993, 2500.5},
		"vendor":          `{"beneficiaryAccountNo":"1234567890","responseCode":"2001800"}`,
		"signature_valid": true,
	}
	b, _ := json.Marshal(Payload(in))
	got := string(b)
	for _, keep := range []string{`"******6655"`, `"S*** E*****"`, `"s***@example.com"`, `"150000.00"`,
		`"TRF20261113000468"`, `9007199254740993`, `2500.5`, `\"******7890\"`, `\"responseCode\":\"2001800\"`,
		`"signature_valid":true`} {
		if !strings.Contains(got, keep) {
			t.Errorf("Payload = %s, want %s", got, keep)
		}
	}
	for _, gone := range []string{"9988776655", "Siti Erlina", "siti@example.com", "1234567890"} {
		if strings.Contains(got, gone) {
			t.Errorf("Payload = %s still holds %q", got, gone)
		}
	}
}

func TestPayload_RootJSONStringStaysAString(t *testing.T) {
	SetEnabled(true)
	got, ok := Payload(`{"accountNo":"9988776655","remark":"gaji"}`).(string)
	if !ok || !strings.Contains(got, `"******6655"`) || strings.Contains(got, "9988776655") {
		t.Fatalf("Payload(json string) = %#v, want a masked JSON string", got)
	}
}

func TestPayload_OffReturnsTheSameValue(t *testing.T) {
	SetEnabled(false)
	t.Cleanup(func() { SetEnabled(true) })
	in := map[string]any{"beneficiaryAccountNo": "9988776655"}
	if got := Payload(in); !reflect.DeepEqual(got, in) {
		t.Fatalf("masking off: Payload = %#v, want the input unchanged", got)
	}
	if Payload(nil) != nil {
		t.Fatal("Payload(nil) must be nil")
	}
}

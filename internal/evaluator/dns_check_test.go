package evaluator

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

// MockDNSResolver implements DNSResolver for testing
type MockDNSResolver struct {
	ResolveFunc func(ctx context.Context, query string, recordType DNSRecordType) (DNSResult, error)
}

func (m *MockDNSResolver) Resolve(ctx context.Context, query string, recordType DNSRecordType) (DNSResult, error) {
	if m.ResolveFunc != nil {
		return m.ResolveFunc(ctx, query, recordType)
	}
	// Default successful resolution
	return DNSResult{
		Records:       []string{"1.2.3.4"},
		TTL:           300,
		Authoritative: false,
		Rcode:         "NOERROR",
		Truncated:     false,
	}, nil
}

func TestNewDNSCheck(t *testing.T) {
	check, err := NewDNSCheck(
		"test-check",
		"example.com",
		RecordTypeA,
		"8.8.8.8",
		53,
		1*time.Minute,
		5*time.Second,
		IPStackAny,
		"prod",
		[]string{"dns", "test"},
		"",
	)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if check.Name != "test-check" {
		t.Errorf("expected name 'test-check', got %q", check.Name)
	}

	if check.Environment != "prod" {
		t.Errorf("expected environment 'prod', got %q", check.Environment)
	}

	if check.Interval != 1*time.Minute {
		t.Errorf("expected interval 1m, got %v", check.Interval)
	}

	if len(check.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(check.Tags))
	}
}

func TestNewDNSCheck_WithAssertion(t *testing.T) {
	check, err := NewDNSCheck(
		"test-check",
		"example.com",
		RecordTypeA,
		"8.8.8.8",
		53,
		1*time.Minute,
		5*time.Second,
		IPStackAny,
		"prod",
		[]string{"dns"},
		`rcode == "NOERROR" && record_count > 0`,
	)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	evaluator, ok := check.Evaluator.(*DNSCheckEvaluator)
	if !ok {
		t.Fatal("expected DNSCheckEvaluator")
	}

	if evaluator.assertionProgram == nil {
		t.Error("expected assertion program to be compiled")
	}
}

func TestNewDNSCheck_InvalidAssertion(t *testing.T) {
	_, err := NewDNSCheck(
		"test-check",
		"example.com",
		RecordTypeA,
		"8.8.8.8",
		53,
		1*time.Minute,
		5*time.Second,
		IPStackAny,
		"prod",
		[]string{},
		"invalid assertion syntax &&",
	)

	if err == nil {
		t.Fatal("expected error for invalid assertion, got nil")
	}
}

func TestNewDNSCheckEvaluator_InvalidQuery(t *testing.T) {
	_, err := NewDNSCheckEvaluator(
		"",
		RecordTypeA,
		"8.8.8.8",
		53,
		5*time.Second,
		IPStackAny,
		"",
	)

	if err == nil {
		t.Fatal("expected error for empty query, got nil")
	}
}

func TestNewDNSCheckEvaluator_InvalidRecordType(t *testing.T) {
	_, err := NewDNSCheckEvaluator(
		"example.com",
		"INVALID",
		"8.8.8.8",
		53,
		5*time.Second,
		IPStackAny,
		"",
	)

	if err == nil {
		t.Fatal("expected error for invalid record type, got nil")
	}
}

func TestNewDNSCheckEvaluator_InvalidHost(t *testing.T) {
	_, err := NewDNSCheckEvaluator(
		"example.com",
		RecordTypeA,
		"",
		53,
		5*time.Second,
		IPStackAny,
		"",
	)

	if err == nil {
		t.Fatal("expected error for empty host, got nil")
	}
}

func TestNewDNSCheckEvaluator_InvalidPort(t *testing.T) {
	testCases := []struct {
		name string
		port int
	}{
		{"negative port", -1},
		{"port too large", 70000},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewDNSCheckEvaluator(
				"example.com",
				RecordTypeA,
				"8.8.8.8",
				tc.port,
				5*time.Second,
				IPStackAny,
				"",
			)

			if err == nil {
				t.Fatalf("expected error for %s, got nil", tc.name)
			}
		})
	}
}

func TestNewDNSCheckEvaluator_PortDefaults(t *testing.T) {
	evaluator, err := NewDNSCheckEvaluator(
		"example.com",
		RecordTypeA,
		"8.8.8.8",
		0,
		5*time.Second,
		IPStackAny,
		"",
	)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if evaluator.Port != DefaultDNSPort {
		t.Errorf("expected port to default to %d, got %d", DefaultDNSPort, evaluator.Port)
	}
}

func TestNewDNSCheckEvaluator_InvalidTimeout(t *testing.T) {
	testCases := []struct {
		name    string
		timeout time.Duration
	}{
		{"zero timeout", 0},
		{"negative timeout", -1 * time.Second},
		{"timeout too large", 35 * time.Second},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewDNSCheckEvaluator(
				"example.com",
				RecordTypeA,
				"8.8.8.8",
				53,
				tc.timeout,
				IPStackAny,
				"",
			)

			if err == nil {
				t.Fatalf("expected error for %s, got nil", tc.name)
			}
		})
	}
}

func TestNewDNSCheckEvaluator_InvalidIPStack(t *testing.T) {
	_, err := NewDNSCheckEvaluator(
		"example.com",
		RecordTypeA,
		"8.8.8.8",
		53,
		5*time.Second,
		"invalid",
		"",
	)

	if err == nil {
		t.Fatal("expected error for invalid IP stack, got nil")
	}
}

func TestDNSCheck_EvaluateSuccess(t *testing.T) {
	evaluator, err := NewDNSCheckEvaluator(
		"example.com",
		RecordTypeA,
		"8.8.8.8",
		53,
		5*time.Second,
		IPStackAny,
		"",
	)
	if err != nil {
		t.Fatalf("failed to create evaluator: %v", err)
	}

	// Use mock resolver
	evaluator.resolver = &MockDNSResolver{
		ResolveFunc: func(ctx context.Context, host string, recordType DNSRecordType) (DNSResult, error) {
			return DNSResult{
				Records:       []string{"1.2.3.4"},
				TTL:           300,
				Authoritative: false,
				Rcode:         "NOERROR",
				Truncated:     false,
			}, nil
		},
	}

	result := evaluator.Evaluate(context.Background())

	if result.Err != nil {
		t.Errorf("expected no error, got %v", result.Err)
	}

	if !result.Pass {
		t.Error("expected check to pass")
	}

	if result.Latency == 0 {
		t.Error("expected non-zero latency")
	}
}

func TestDNSCheck_EvaluateMultipleRecords(t *testing.T) {
	evaluator, err := NewDNSCheckEvaluator(
		"example.com",
		RecordTypeA,
		"8.8.8.8",
		53,
		5*time.Second,
		IPStackAny,
		"",
	)
	if err != nil {
		t.Fatalf("failed to create evaluator: %v", err)
	}

	evaluator.resolver = &MockDNSResolver{
		ResolveFunc: func(ctx context.Context, host string, recordType DNSRecordType) (DNSResult, error) {
			return DNSResult{
				Records:       []string{"1.2.3.4", "5.6.7.8", "9.10.11.12"},
				TTL:           300,
				Authoritative: false,
				Rcode:         "NOERROR",
				Truncated:     false,
			}, nil
		},
	}

	result := evaluator.Evaluate(context.Background())

	if result.Err != nil {
		t.Errorf("expected no error, got %v", result.Err)
	}

	if !result.Pass {
		t.Error("expected check to pass with multiple records")
	}
}

func TestDNSCheck_EvaluateNXDOMAIN(t *testing.T) {
	evaluator, err := NewDNSCheckEvaluator(
		"nonexistent.example.com",
		RecordTypeA,
		"8.8.8.8",
		53,
		5*time.Second,
		IPStackAny,
		"",
	)
	if err != nil {
		t.Fatalf("failed to create evaluator: %v", err)
	}

	evaluator.resolver = &MockDNSResolver{
		ResolveFunc: func(ctx context.Context, host string, recordType DNSRecordType) (DNSResult, error) {
			return DNSResult{
				Records:       []string{},
				TTL:           0,
				Authoritative: true,
				Rcode:         "NXDOMAIN",
				Truncated:     false,
			}, nil
		},
	}

	result := evaluator.Evaluate(context.Background())

	if result.Err != nil {
		t.Errorf("expected no error, got %v", result.Err)
	}

	if result.Pass {
		t.Error("expected check to fail with NXDOMAIN and no records")
	}
}

func TestDNSCheck_EvaluateWithAssertionPass(t *testing.T) {
	evaluator, err := NewDNSCheckEvaluator(
		"example.com",
		RecordTypeA,
		"8.8.8.8",
		53,
		5*time.Second,
		IPStackAny,
		`rcode == "NOERROR" && record_count > 0`,
	)
	if err != nil {
		t.Fatalf("failed to create evaluator: %v", err)
	}

	evaluator.resolver = &MockDNSResolver{
		ResolveFunc: func(ctx context.Context, host string, recordType DNSRecordType) (DNSResult, error) {
			return DNSResult{
				Records:       []string{"1.2.3.4"},
				TTL:           300,
				Authoritative: false,
				Rcode:         "NOERROR",
				Truncated:     false,
			}, nil
		},
	}

	result := evaluator.Evaluate(context.Background())

	if result.Err != nil {
		t.Errorf("expected no error, got %v", result.Err)
	}

	if !result.Pass {
		t.Error("expected check to pass with assertion")
	}
}

func TestDNSCheck_EvaluateWithAssertionFail(t *testing.T) {
	evaluator, err := NewDNSCheckEvaluator(
		"example.com",
		RecordTypeA,
		"8.8.8.8",
		53,
		5*time.Second,
		IPStackAny,
		`record_count > 5`,
	)
	if err != nil {
		t.Fatalf("failed to create evaluator: %v", err)
	}

	evaluator.resolver = &MockDNSResolver{
		ResolveFunc: func(ctx context.Context, host string, recordType DNSRecordType) (DNSResult, error) {
			return DNSResult{
				Records:       []string{"1.2.3.4"},
				TTL:           300,
				Authoritative: false,
				Rcode:         "NOERROR",
				Truncated:     false,
			}, nil
		},
	}

	result := evaluator.Evaluate(context.Background())

	if result.Err != nil {
		t.Errorf("expected no error, got %v", result.Err)
	}

	if result.Pass {
		t.Error("expected check to fail when assertion fails")
	}
}

func TestDNSCheck_EvaluateTimeout(t *testing.T) {
	evaluator, err := NewDNSCheckEvaluator(
		"example.com",
		RecordTypeA,
		"8.8.8.8",
		53,
		5*time.Second,
		IPStackAny,
		"",
	)
	if err != nil {
		t.Fatalf("failed to create evaluator: %v", err)
	}

	evaluator.resolver = &MockDNSResolver{
		ResolveFunc: func(ctx context.Context, host string, recordType DNSRecordType) (DNSResult, error) {
			return DNSResult{}, errors.New("i/o timeout")
		},
	}

	result := evaluator.Evaluate(context.Background())

	if result.Err == nil {
		t.Error("expected error for timeout")
	}

	if result.Pass {
		t.Error("expected check to fail on timeout")
	}
}

func TestDNSCheck_EvaluateContextCancellation(t *testing.T) {
	evaluator, err := NewDNSCheckEvaluator(
		"example.com",
		RecordTypeA,
		"8.8.8.8",
		53,
		5*time.Second,
		IPStackAny,
		"",
	)
	if err != nil {
		t.Fatalf("failed to create evaluator: %v", err)
	}

	evaluator.resolver = &MockDNSResolver{
		ResolveFunc: func(ctx context.Context, host string, recordType DNSRecordType) (DNSResult, error) {
			return DNSResult{}, context.Canceled
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result := evaluator.Evaluate(ctx)

	if result.Err == nil {
		t.Error("expected error for cancelled context")
	}
}

func TestDNSCheck_CELVariables(t *testing.T) {
	evaluator, err := NewDNSCheckEvaluator(
		"example.com",
		RecordTypeA,
		"8.8.8.8",
		53,
		5*time.Second,
		IPStackAny,
		`records == ["1.2.3.4", "5.6.7.8"] &&
		 record_count == 2 &&
		 ttl == 600 &&
		 authoritative == true &&
		 rcode == "NOERROR" &&
		 truncated == false`,
	)
	if err != nil {
		t.Fatalf("failed to create evaluator: %v", err)
	}

	evaluator.resolver = &MockDNSResolver{
		ResolveFunc: func(ctx context.Context, host string, recordType DNSRecordType) (DNSResult, error) {
			return DNSResult{
				Records:       []string{"1.2.3.4", "5.6.7.8"},
				TTL:           600,
				Authoritative: true,
				Rcode:         "NOERROR",
				Truncated:     false,
			}, nil
		},
	}

	result := evaluator.Evaluate(context.Background())

	if result.Err != nil {
		t.Errorf("expected no error, got %v", result.Err)
	}

	if !result.Pass {
		t.Error("expected check to pass with all CEL variables matching")
	}
}

func TestDNSCheck_RecordTypes(t *testing.T) {
	testCases := []struct {
		recordType     DNSRecordType
		expectedRecord string
	}{
		{RecordTypeA, "1.2.3.4"},
		{RecordTypeAAAA, "2001:db8::1"},
		{RecordTypeCNAME, "example.org"},
		{RecordTypeMX, "10 mail.example.com"},
		{RecordTypeTXT, "v=spf1 include:example.com ~all"},
		{RecordTypeNS, "ns1.example.com"},
		{RecordTypeSOA, "ns1.example.com admin.example.com 2024010101 3600 600 604800 86400"},
	}

	for _, tc := range testCases {
		t.Run(string(tc.recordType), func(t *testing.T) {
			evaluator, err := NewDNSCheckEvaluator(
				"example.com",
				tc.recordType,
				"8.8.8.8",
				53,
				5*time.Second,
				IPStackAny,
				"",
			)
			if err != nil {
				t.Fatalf("failed to create evaluator: %v", err)
			}

			evaluator.resolver = &MockDNSResolver{
				ResolveFunc: func(ctx context.Context, host string, recordType DNSRecordType) (DNSResult, error) {
					return DNSResult{
						Records:       []string{tc.expectedRecord},
						TTL:           300,
						Authoritative: false,
						Rcode:         "NOERROR",
						Truncated:     false,
					}, nil
				},
			}

			result := evaluator.Evaluate(context.Background())

			if result.Err != nil {
				t.Errorf("expected no error, got %v", result.Err)
			}

			if !result.Pass {
				t.Errorf("expected check to pass for %s record type", tc.recordType)
			}
		})
	}
}

func TestDNSCheck_IPStack(t *testing.T) {
	testCases := []struct {
		name    string
		ipStack IPStack
	}{
		{"IPv4", IPStackIPv4},
		{"IPv6", IPStackIPv6},
		{"Any", IPStackAny},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			evaluator, err := NewDNSCheckEvaluator(
				"example.com",
				RecordTypeA,
				"8.8.8.8",
				53,
				5*time.Second,
				tc.ipStack,
				"",
			)
			if err != nil {
				t.Fatalf("failed to create evaluator: %v", err)
			}

			if evaluator.IPStack != tc.ipStack {
				t.Errorf("expected IP stack %q, got %q", tc.ipStack, evaluator.IPStack)
			}
		})
	}
}

func TestDNSCheck_LogAttrsOnPass(t *testing.T) {
	evaluator, err := NewDNSCheckEvaluator("example.com", RecordTypeA, "8.8.8.8", 53, 5*time.Second, IPStackAny, "")
	if err != nil {
		t.Fatalf("failed to create evaluator: %v", err)
	}

	evaluator.resolver = &MockDNSResolver{
		ResolveFunc: func(ctx context.Context, host string, recordType DNSRecordType) (DNSResult, error) {
			return DNSResult{
				Records:       []string{"1.2.3.4", "5.6.7.8"},
				TTL:           300,
				Authoritative: true,
				Rcode:         "NOERROR",
				Truncated:     false,
			}, nil
		},
	}

	result := evaluator.Evaluate(context.Background())

	if !result.Pass {
		t.Error("expected check to pass")
	}

	if got := logAttr(t, result, "dnsRcode").String(); got != "NOERROR" {
		t.Errorf("expected dnsRcode NOERROR, got %q", got)
	}

	if got := logAttr(t, result, "dnsRecordCount").Int64(); got != 2 {
		t.Errorf("expected dnsRecordCount 2, got %d", got)
	}

	if got := logAttr(t, result, "dnsRecords").Any(); !reflect.DeepEqual(got, []string{"1.2.3.4", "5.6.7.8"}) {
		t.Errorf("expected dnsRecords to hold both records, got %v", got)
	}

	if got := logAttr(t, result, "dnsTTL").Uint64(); got != 300 {
		t.Errorf("expected dnsTTL 300, got %d", got)
	}

	if !logAttr(t, result, "dnsAuthoritative").Bool() {
		t.Error("expected dnsAuthoritative to be true")
	}

	if logAttr(t, result, "dnsTruncated").Bool() {
		t.Error("expected dnsTruncated to be false")
	}
}

func TestDNSCheck_LogAttrsOnNonPass(t *testing.T) {
	evaluator, err := NewDNSCheckEvaluator("example.com", RecordTypeA, "8.8.8.8", 53, 5*time.Second, IPStackAny, "")
	if err != nil {
		t.Fatalf("failed to create evaluator: %v", err)
	}

	evaluator.resolver = &MockDNSResolver{
		ResolveFunc: func(ctx context.Context, host string, recordType DNSRecordType) (DNSResult, error) {
			return DNSResult{Records: []string{}, Rcode: "NXDOMAIN"}, nil
		},
	}

	result := evaluator.Evaluate(context.Background())

	if result.Pass {
		t.Error("expected check to not pass")
	}

	if got := logAttr(t, result, "dnsRcode").String(); got != "NXDOMAIN" {
		t.Errorf("expected dnsRcode NXDOMAIN, got %q", got)
	}

	if got := logAttr(t, result, "dnsRecordCount").Int64(); got != 0 {
		t.Errorf("expected dnsRecordCount 0, got %d", got)
	}
}

func TestDNSCheck_LogAttrsOnQueryFailure(t *testing.T) {
	evaluator, err := NewDNSCheckEvaluator("example.com", RecordTypeA, "8.8.8.8", 53, 5*time.Second, IPStackAny, "")
	if err != nil {
		t.Fatalf("failed to create evaluator: %v", err)
	}

	evaluator.resolver = &MockDNSResolver{
		ResolveFunc: func(ctx context.Context, host string, recordType DNSRecordType) (DNSResult, error) {
			return DNSResult{}, errors.New("i/o timeout")
		},
	}

	result := evaluator.Evaluate(context.Background())

	if result.Err == nil {
		t.Fatal("expected an error")
	}

	// There is no response to describe, so no DNS attributes are attached.
	if len(result.LogAttrs) != 0 {
		t.Errorf("expected no log attributes, got %v", result.LogAttrs)
	}
}

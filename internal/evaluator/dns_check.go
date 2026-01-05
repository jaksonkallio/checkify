package evaluator

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/google/cel-go/cel"
	"github.com/miekg/dns"
)

const (
	DefaultDNSTimeout = 5 * time.Second
	MaxDNSTimeout     = 30 * time.Second
	DefaultDNSPort    = 53
)

type DNSRecordType string

const (
	RecordTypeA     DNSRecordType = "A"
	RecordTypeAAAA  DNSRecordType = "AAAA"
	RecordTypeCNAME DNSRecordType = "CNAME"
	RecordTypeMX    DNSRecordType = "MX"
	RecordTypeTXT   DNSRecordType = "TXT"
	RecordTypeNS    DNSRecordType = "NS"
	RecordTypeSOA   DNSRecordType = "SOA"
)

// DNSResult contains the results of a DNS query
type DNSResult struct {
	Records       []string
	TTL           uint32
	Authoritative bool
	Rcode         string
	Truncated     bool
}

// DNSResolver interface allows mocking DNS queries for testing
type DNSResolver interface {
	Resolve(ctx context.Context, query string, recordType DNSRecordType) (DNSResult, error)
}

// DNSCheckEvaluator implements the CheckEvaluator interface for DNS checks
type DNSCheckEvaluator struct {
	Query            string
	RecordType       DNSRecordType
	Host             string
	Port             int
	Timeout          time.Duration
	IPStack          IPStack
	assertionProgram cel.Program
	resolver         DNSResolver
}

// NewDNSCheckEvaluator creates a new DNS check evaluator with validation
func NewDNSCheckEvaluator(
	query string,
	recordType DNSRecordType,
	host string,
	port int,
	timeout time.Duration,
	ipStack IPStack,
	assertion string,
) (*DNSCheckEvaluator, error) {
	if len(query) == 0 {
		return nil, fmt.Errorf("query cannot be empty")
	}

	switch recordType {
	case RecordTypeA, RecordTypeAAAA, RecordTypeCNAME, RecordTypeMX, RecordTypeTXT, RecordTypeNS, RecordTypeSOA:
	default:
		return nil, fmt.Errorf("unsupported record type %q", recordType)
	}

	if len(host) == 0 {
		return nil, fmt.Errorf("host cannot be empty")
	}

	if port == 0 {
		port = DefaultDNSPort
	}
	if port < 1 || port > 65535 {
		return nil, fmt.Errorf("port must be between 1 and 65535")
	}

	if timeout <= 0 || timeout > MaxDNSTimeout {
		return nil, fmt.Errorf("timeout must be greater than 0 and less than %s", MaxDNSTimeout)
	}

	if ipStack == "" {
		ipStack = IPStackDefault
	}
	switch ipStack {
	case IPStackIPv4, IPStackIPv6, IPStackAny:
	default:
		return nil, fmt.Errorf("ipstack must be one of: ipv4, ipv6, any")
	}

	// Compile CEL assertion if provided
	var assertionProgram cel.Program
	if assertion != "" {
		env, err := cel.NewEnv(
			cel.Variable("records", cel.ListType(cel.StringType)),
			cel.Variable("record_count", cel.IntType),
			cel.Variable("ttl", cel.IntType),
			cel.Variable("authoritative", cel.BoolType),
			cel.Variable("rcode", cel.StringType),
			cel.Variable("truncated", cel.BoolType),
		)
		if err != nil {
			return nil, fmt.Errorf("create CEL environment: %w", err)
		}

		ast, issues := env.Compile(assertion)
		if issues != nil && issues.Err() != nil {
			return nil, fmt.Errorf("compile CEL expression: %w", issues.Err())
		}

		prg, err := env.Program(ast)
		if err != nil {
			return nil, fmt.Errorf("create CEL program: %w", err)
		}

		assertionProgram = prg
	}

	// Create resolver
	resolver := &DefaultDNSResolver{
		Server:  host,
		Port:    port,
		Timeout: timeout,
		IPStack: ipStack,
	}

	return &DNSCheckEvaluator{
		Query:            query,
		RecordType:       recordType,
		Host:             host,
		Port:             port,
		Timeout:          timeout,
		IPStack:          ipStack,
		assertionProgram: assertionProgram,
		resolver:         resolver,
	}, nil
}

// NewDNSCheck creates a new DNS check with the given parameters
func NewDNSCheck(
	name string,
	query string,
	recordType DNSRecordType,
	host string,
	port int,
	interval time.Duration,
	timeout time.Duration,
	ipStack IPStack,
	environment string,
	tags []string,
	assertion string,
) (Check, error) {
	evaluator, err := NewDNSCheckEvaluator(query, recordType, host, port, timeout, ipStack, assertion)
	if err != nil {
		return Check{}, err
	}

	return Check{
		Name:        name,
		Environment: environment,
		Interval:    interval,
		Tags:        tags,
		Evaluator:   evaluator,
	}, nil
}

// Evaluate performs the DNS check
func (c *DNSCheckEvaluator) Evaluate(ctx context.Context) CheckResult {
	if c.Timeout <= 0 {
		return CheckResult{
			Err:     NewErrCheckTimeout(c.Timeout),
			Latency: 0,
		}
	}

	start := time.Now()
	result, err := c.resolver.Resolve(ctx, c.Query, c.RecordType)
	latency := time.Since(start)

	if err != nil {
		return CheckResult{
			Err:     fmt.Errorf("DNS query failed: %w", err),
			Latency: latency,
		}
	}

	// Default to pass if no assertion
	var pass bool
	if c.assertionProgram != nil {
		assertionPasses, err := c.evaluateAssertion(result)
		if err != nil {
			return CheckResult{
				Err:     fmt.Errorf("evaluate assertion: %w", err),
				Latency: latency,
			}
		}
		pass = assertionPasses
	} else {
		// No assertion: pass if we got a NOERROR response with at least one record
		pass = result.Rcode == "NOERROR" && len(result.Records) > 0
	}

	return CheckResult{
		Pass:    pass,
		Latency: latency,
		Err:     nil,
	}
}

// evaluateAssertion evaluates the CEL assertion with DNS result variables
func (c *DNSCheckEvaluator) evaluateAssertion(result DNSResult) (bool, error) {
	out, _, err := c.assertionProgram.Eval(map[string]any{
		"records":       result.Records,
		"record_count":  int64(len(result.Records)),
		"ttl":           int64(result.TTL),
		"authoritative": result.Authoritative,
		"rcode":         result.Rcode,
		"truncated":     result.Truncated,
	})
	if err != nil {
		return false, fmt.Errorf("evaluate CEL expression: %w", err)
	}

	pass, ok := out.Value().(bool)
	if !ok {
		return false, fmt.Errorf("CEL expression did not return a boolean value")
	}

	return pass, nil
}

// DefaultDNSResolver implements DNSResolver using github.com/miekg/dns
type DefaultDNSResolver struct {
	Server  string
	Port    int
	Timeout time.Duration
	IPStack IPStack
}

// Resolve performs a DNS query and returns the result
func (r *DefaultDNSResolver) Resolve(ctx context.Context, query string, recordType DNSRecordType) (DNSResult, error) {
	// Ensure query ends with a dot for FQDN
	if !strings.HasSuffix(query, ".") {
		query = query + "."
	}

	// Create DNS message
	msg := new(dns.Msg)
	msg.SetQuestion(query, recordTypeToUint16(recordType))
	msg.RecursionDesired = true

	// Determine network protocol based on IP stack
	network := "udp"
	switch r.IPStack {
	case IPStackIPv4:
		network = "udp4"
	case IPStackIPv6:
		network = "udp6"
	case IPStackAny:
		network = "udp"
	}

	// Create DNS client
	client := &dns.Client{
		Timeout: r.Timeout,
		Net:     network,
	}

	// Build server address
	serverAddr := net.JoinHostPort(r.Server, strconv.Itoa(r.Port))

	// Perform DNS query
	resp, _, err := client.ExchangeContext(ctx, msg, serverAddr)

	if err != nil {
		return DNSResult{}, fmt.Errorf("DNS query exchange failed: %w", err)
	}

	if resp == nil {
		return DNSResult{}, fmt.Errorf("DNS query returned nil response")
	}

	// Extract records
	records := extractRecords(resp.Answer, recordType)

	// Calculate minimum TTL
	var minTTL uint32 = 0
	if len(resp.Answer) > 0 {
		minTTL = resp.Answer[0].Header().Ttl
		for _, rr := range resp.Answer {
			if rr.Header().Ttl < minTTL {
				minTTL = rr.Header().Ttl
			}
		}
	}

	return DNSResult{
		Records:       records,
		TTL:           minTTL,
		Authoritative: resp.Authoritative,
		Rcode:         dns.RcodeToString[resp.Rcode],
		Truncated:     resp.Truncated,
	}, nil
}

// recordTypeToUint16 converts DNSRecordType to uint16 for DNS query
func recordTypeToUint16(recordType DNSRecordType) uint16 {
	switch recordType {
	case RecordTypeA:
		return dns.TypeA
	case RecordTypeAAAA:
		return dns.TypeAAAA
	case RecordTypeCNAME:
		return dns.TypeCNAME
	case RecordTypeMX:
		return dns.TypeMX
	case RecordTypeTXT:
		return dns.TypeTXT
	case RecordTypeNS:
		return dns.TypeNS
	case RecordTypeSOA:
		return dns.TypeSOA
	default:
		return 0
	}
}

// extractRecords extracts record values from DNS answer section
func extractRecords(answers []dns.RR, recordType DNSRecordType) []string {
	var records []string

	for _, rr := range answers {
		switch recordType {
		case RecordTypeA:
			if a, ok := rr.(*dns.A); ok {
				records = append(records, a.A.String())
			}
		case RecordTypeAAAA:
			if aaaa, ok := rr.(*dns.AAAA); ok {
				records = append(records, aaaa.AAAA.String())
			}
		case RecordTypeCNAME:
			if cname, ok := rr.(*dns.CNAME); ok {
				records = append(records, strings.TrimSuffix(cname.Target, "."))
			}
		case RecordTypeMX:
			if mx, ok := rr.(*dns.MX); ok {
				records = append(records, fmt.Sprintf("%d %s", mx.Preference, strings.TrimSuffix(mx.Mx, ".")))
			}
		case RecordTypeTXT:
			if txt, ok := rr.(*dns.TXT); ok {
				records = append(records, strings.Join(txt.Txt, ""))
			}
		case RecordTypeNS:
			if ns, ok := rr.(*dns.NS); ok {
				records = append(records, strings.TrimSuffix(ns.Ns, "."))
			}
		case RecordTypeSOA:
			if soa, ok := rr.(*dns.SOA); ok {
				records = append(records, fmt.Sprintf("%s %s %d %d %d %d %d",
					strings.TrimSuffix(soa.Ns, "."),
					strings.TrimSuffix(soa.Mbox, "."),
					soa.Serial,
					soa.Refresh,
					soa.Retry,
					soa.Expire,
					soa.Minttl,
				))
			}
		}
	}

	return records
}

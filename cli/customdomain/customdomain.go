// Package customdomain calls the custom domain endpoints of the Blaxel API.
// The Go SDK has no custom domain resource, so requests go through the
// generic authenticated client with raw paths.
package customdomain

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"

	blaxel "github.com/blaxel-ai/sdk-go"
	"github.com/blaxel-ai/toolkit/cli/core"
	"golang.org/x/net/publicsuffix"
)

const basePath = "customdomains"

// Domain types accepted by the API.
var DomainTypes = []string{"previews", "applications"}

// Validation methods accepted by the API. The API defaults to "dns".
var ValidationMethods = []string{"dns", "http"}

// CreateOptions describes a custom domain to create. An empty
// ValidationMethod is not sent, so the API applies its default.
type CreateOptions struct {
	Name                    string
	DomainType              string
	SubjectAlternativeNames []string
	ValidationMethod        string
}

// CreateRequest is the body of POST /customdomains.
type CreateRequest struct {
	Metadata CreateMetadata `json:"metadata"`
	Spec     CreateSpec     `json:"spec"`
}

type CreateMetadata struct {
	Name string `json:"name"`
}

type CreateSpec struct {
	DomainType              string   `json:"domainType"`
	SubjectAlternativeNames []string `json:"subjectAlternativeNames,omitempty"`
	ValidationMethod        string   `json:"validationMethod,omitempty"`
}

// BuildCreateRequest turns CLI options into the API request body.
func BuildCreateRequest(opts CreateOptions) (CreateRequest, error) {
	name := strings.TrimSpace(opts.Name)
	if name == "" {
		return CreateRequest{}, errors.New("a domain name is required")
	}
	if !contains(DomainTypes, opts.DomainType) {
		return CreateRequest{}, fmt.Errorf("--type must be one of %s (got %q)", strings.Join(DomainTypes, ", "), opts.DomainType)
	}
	var sans []string
	for _, san := range opts.SubjectAlternativeNames {
		if san = strings.TrimSpace(san); san != "" {
			sans = append(sans, san)
		}
	}
	return CreateRequest{
		Metadata: CreateMetadata{Name: name},
		Spec: CreateSpec{
			DomainType:              opts.DomainType,
			SubjectAlternativeNames: sans,
			ValidationMethod:        opts.ValidationMethod,
		},
	}, nil
}

// List returns every custom domain of the workspace.
func List(ctx context.Context, c *blaxel.Client) ([]any, error) {
	var domains []any
	if err := c.Get(ctx, basePath, nil, &domains); err != nil {
		return nil, err
	}
	return domains, nil
}

// Get returns a single custom domain.
func Get(ctx context.Context, c *blaxel.Client, name string) (map[string]any, error) {
	var domain map[string]any
	if err := c.Get(ctx, domainPath(name), nil, &domain); err != nil {
		return nil, err
	}
	return domain, nil
}

// Create creates a custom domain.
func Create(ctx context.Context, c *blaxel.Client, req CreateRequest) (map[string]any, error) {
	var domain map[string]any
	if err := c.Post(ctx, basePath, req, &domain); err != nil {
		return nil, err
	}
	return domain, nil
}

// Verify asks the API to check the DNS records of a custom domain.
func Verify(ctx context.Context, c *blaxel.Client, name string) (map[string]any, error) {
	var domain map[string]any
	if err := c.Post(ctx, domainPath(name)+"/verify", nil, &domain); err != nil {
		return nil, err
	}
	return domain, nil
}

// Delete deletes a custom domain.
func Delete(ctx context.Context, c *blaxel.Client, name string) error {
	return c.Delete(ctx, domainPath(name), nil, nil)
}

func domainPath(name string) string {
	return basePath + "/" + url.PathEscape(name)
}

// Record is one DNS record to publish for a custom domain.
type Record struct {
	Type    string `json:"type" yaml:"type"`
	Name    string `json:"name" yaml:"name"`
	Value   string `json:"value" yaml:"value"`
	Purpose string `json:"purpose" yaml:"purpose"`
}

const (
	RecordTypeCNAME = "CNAME"
	RecordTypeAlias = "ALIAS/ANAME"
	RecordTypeTXT   = "TXT"

	PurposeRouting     = "routing"
	PurposeCertificate = "certificate validation"
	PurposeVerify      = "domain verification"
	PurposeApexHTTP    = "apex HTTP validation"
)

const cfChallengePrefix = "_cf-challenge."

// Records lists the DNS records of a custom domain from its spec.
// Routing records on an apex name are reported as ALIAS/ANAME: an apex
// cannot carry a CNAME.
func Records(domain map[string]any) []Record {
	spec, _ := domain["spec"].(map[string]any)
	txt := stringMap(spec["txtRecords"])
	cname := stringMap(spec["cnameRecords"])

	var records []Record
	for _, name := range sortedKeys(txt) {
		purpose := PurposeVerify
		if strings.HasPrefix(name, cfChallengePrefix) {
			purpose = PurposeApexHTTP
		}
		records = append(records, Record{Type: RecordTypeTXT, Name: name, Value: txt[name], Purpose: purpose})
	}
	var validation []Record
	for _, name := range sortedKeys(cname) {
		if strings.HasPrefix(name, "_") {
			validation = append(validation, Record{Type: RecordTypeCNAME, Name: name, Value: cname[name], Purpose: PurposeCertificate})
			continue
		}
		recordType := RecordTypeCNAME
		if isApex(name, txt) {
			recordType = RecordTypeAlias
		}
		records = append(records, Record{Type: recordType, Name: name, Value: cname[name], Purpose: PurposeRouting})
	}
	return append(records, validation...)
}

// isApex reports whether name is a registrable domain (example.com,
// example.co.uk). The API marks apex names of HTTP-validated domains with a
// _cf-challenge TXT record.
func isApex(name string, txt map[string]string) bool {
	if _, ok := txt[cfChallengePrefix+name]; ok {
		return true
	}
	name = strings.TrimSuffix(strings.ToLower(name), ".")
	if strings.HasPrefix(name, "*.") {
		return false
	}
	apex, err := publicsuffix.EffectiveTLDPlusOne(name)
	return err == nil && apex == name
}

// DescribeError turns an API error into an error carrying the API's message
// and status code. Other errors are returned unchanged.
func DescribeError(err error) error {
	var apiErr *blaxel.Error
	if !errors.As(err, &apiErr) || apiErr.Response == nil {
		return err
	}
	status := apiErr.StatusCode
	message := apiMessage(apiErr.RawJSON())
	if message == "" {
		return core.MarkExpectedHTTPError(err, status)
	}
	return core.MarkExpectedHTTPError(fmt.Errorf("%s (HTTP %d)", message, status), status)
}

func apiMessage(raw string) string {
	var body struct {
		Message string `json:"message"`
		Error   string `json:"error"`
	}
	if json.Unmarshal([]byte(raw), &body) != nil {
		return ""
	}
	if body.Message != "" {
		return body.Message
	}
	return body.Error
}

func stringMap(v any) map[string]string {
	out := map[string]string{}
	m, _ := v.(map[string]any)
	for k, val := range m {
		if s, ok := val.(string); ok {
			out[k] = s
		}
	}
	return out
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func contains(values []string, v string) bool {
	for _, value := range values {
		if value == v {
			return true
		}
	}
	return false
}

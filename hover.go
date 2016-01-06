package hover

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"golang.org/x/net/context"
	"golang.org/x/net/context/ctxhttp"
)

// RecordType is a string of DNS record type's name. See the constants of this
// package for some helpful constants, but any string is allowed by the API.
type RecordType string

// UnmarshalJSON is for making it easy to JSON parse RecordType
func (rt *RecordType) UnmarshalJSON(b []byte) error {
	if b[0] == '"' && b[len(b)-1] == '"' {
		b = b[1 : len(b)-1]
	} else {
		return fmt.Errorf("unable to parse a RecordType from %#v", string(b))
	}
	*rt = RecordType(string(b))
	return nil
}

// Some of the available DNS record types.
const (
	A     = RecordType("A")
	AAAA  = RecordType("AAAA")
	CAA   = RecordType("CAA")
	CNAME = RecordType("CNAME")
	MX    = RecordType("MX")
	SRV   = RecordType("SRV")
	TXT   = RecordType("TXT")
)

const defaultURL = "https://www.hover.com/api"

// Client provides methods to access the unofficial Hover DNS API
type Client struct {
	hc   *http.Client
	cook *http.Cookie
}

// InvalidLogin is returned from Login when the credentials don't work
type InvalidLogin string

func (il InvalidLogin) Error() string {
	return string(il)
}

// Login takes a Hover username and password and returns the login cookie value
func Login(ctx context.Context, hc *http.Client, username, password string) (*http.Cookie, error) {
	v := make(url.Values)
	v.Set("username", username)
	v.Set("password", password)

	r, err := ctxhttp.Get(ctx, hc, fmt.Sprintf("%s/login?%s", defaultURL, v.Encode()))

	if err != nil {
		return nil, err
	}
	if r.StatusCode != 200 {
		return nil, InvalidLogin(fmt.Sprintf("login HTTP status code was %d", r.StatusCode))
	}
	var c *http.Cookie
	for _, cook := range r.Cookies() {
		if cook.Name == "hoverauth" && cook.Value != "" {
			c = cook
			break
		}
	}
	if c == nil {
		return nil, InvalidLogin("unable to find 'hoverauth' cookie with data in response")
	}
	return c, nil
}

// NewClient takes an http.Client with the hoverauth cookie in its cookiejar
func NewClient(hc *http.Client, loginCookie *http.Cookie) *Client {
	return &Client{hc: hc, cook: loginCookie}
}

type hoverResp struct {
	Succeeded bool   `json:"succeeded"`
	ErrorCode string `json:"error_code"`
	Error     string `json:"error"`
}

// User contains information on a domain's Hover user
type User struct {
	Email          string  `json:"email"`
	EmailSecondary string  `json:"email_secondary"`
	Billing        Billing `json:"billing"`
}

// Billing contains information on how a domain's Hover user has set up
// billing
type Billing struct {
	PayMode     string    `json:"pay_mode"` // TODO: enum?
	CardNumber  string    `json:"card_number"`
	CardExpires YearMonth `json:"card_expires"`

	FirstName  string `json:"first_name"`
	LastName   string `json:"last_name"`
	Address1   string `json:"address1"`
	Address2   string `json:"address2"`
	City       string `json:"city"`
	State      string `json:"state"`
	Country    string `json:"country"`
	PostalCode string `json:"postal_code"`
	Phone      string `json:"phone"`
}

// Contact contains information about how to contact a person or organization
type Contact struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	OrgName   string `json:"org_name"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
	Fax       string `json:"fax"`
	City      string `json:"city"`
	State     string `json:"state"`
	ZIPCode   string `json:"zip"`
	Country   string `json:"country"`
	Address1  string `json:"address1"`
	Address2  string `json:"address2"`
	Address3  string `json:"address3"`
}

// Contacts contains contact information for a domain.
type Contacts struct {
	Admin Contact `json:"admin"`
	Owner Contact `json:"owner"`
	Tech  Contact `json:"tech"`
}

// Domain represents a domain and its information.
type Domain struct {
	ID         DomainID `json:"id"`
	Status     string   `json:"status"` // TODO: make enum
	DomainName string   `json:"domain_name"`

	RenewalDate Date `json:"renewal_date"`
	DisplayDate Date `json:"display_date"`

	Glue           map[string]interface{} `json:"glue"`
	HoverUser      User                   `json:"hover_user"`
	Nameservers    []string               `json:"nameservers"`
	Renewable      bool                   `json:"renewable"`
	AutoRenew      bool                   `json:"auto_renew"`
	Locked         bool                   `json:"locked"`
	WhoisPrivacy   bool                   `json:"whois_privacy"`
	NumEmails      int                    `json:"num_emails"`
	Contacts       Contacts               `json:"contacts"`
	RegisteredDate Date                   `json:"registered_date"`
}

// DomainID is a string that identifies a specific Domain
type DomainID string

// UnmarshalJSON is for making it easy to JSON parse DomainID
func (dri *DomainID) UnmarshalJSON(b []byte) error {
	if b[0] == '"' && b[len(b)-1] == '"' {
		b = b[1 : len(b)-1]
	} else {
		return fmt.Errorf("unable to parse a DNSRecordID from %#v", string(b))
	}
	*dri = DomainID(string(b))
	return nil
}

// DNSRecordID is a string that identifies a specific DNSRecord in a DNSDomain
type DNSRecordID string

// UnmarshalJSON is for making it easy to JSON parse DNSRecordID
func (dri *DNSRecordID) UnmarshalJSON(b []byte) error {
	if b[0] == '"' && b[len(b)-1] == '"' {
		b = b[1 : len(b)-1]
	} else {
		return fmt.Errorf("unable to parse a DNSRecordID from %#v", string(b))
	}
	*dri = DNSRecordID(string(b))
	return nil
}

// DNSRecord represents a DNS record for a given DNSDomain
type DNSRecord struct {
	ID        DNSRecordID `json:"id,omitempty"`
	Type      RecordType  `json:"type,omitempty"`
	Name      string      `json:"name,omitempty"`
	Content   string      `json:"content,omitempty"`
	TTL       TTL         `json:"ttl,omitempty"`
	IsDefault bool        `json:"is_default,omitempty"`
	CanRevert bool        `json:"can_revert,omitempty"`
}

// DNSDomain contains the actual DNS information for a Domain
type DNSDomain struct {
	ID         DomainID     `json:"id"`
	DomainName string       `json:"domain_name"`
	Active     bool         `json:"active"`
	Entries    []*DNSRecord `json:"entries"`
}

// TTL is to convert JSON ints representing time-to-live seconds into a
// time.Duration.
type TTL time.Duration

// UnmarshalJSON is for making it easy to JSON parse TTL
func (ttl *TTL) UnmarshalJSON(b []byte) error {
	val, err := strconv.Atoi(string(b))
	if err != nil {
		return fmt.Errorf("unable to parse a TTL from %s", string(b))
	}
	*ttl = TTL(time.Duration(val) * time.Second)
	return nil
}

// Date is for parsing year, month, and day date strings.
type Date struct {
	time.Time
}

// UnmarshalJSON is for making it easy to JSON parse Date
func (d *Date) UnmarshalJSON(b []byte) error {
	if b[0] == '"' && b[len(b)-1] == '"' {
		b = b[1 : len(b)-1]
	} else {
		return fmt.Errorf("unable to parse a Date from %#v", string(b))
	}
	var err error
	d.Time, err = time.Parse("2006-01-02", string(b))
	return fmt.Errorf("unable to parse the Date string: %s", err)
}

// YearMonth is for parsing year and month date strings. Used only in the
// Billing struct
type YearMonth struct {
	time.Time
}

// UnmarshalJSON is for making it easy to JSON parse YearMonth
func (ym *YearMonth) UnmarshalJSON(b []byte) error {
	if b[0] == '"' && b[len(b)-1] == '"' {
		b = b[1 : len(b)-1]
	} else {
		return fmt.Errorf("unable to parse a YearMonth from %#v", string(b))
	}
	var err error
	ym.Time, err = time.Parse("2006/01", string(b))
	return fmt.Errorf("unable to parse the YearMonth string: %s", err)
}

// APIError is the error type used for errors from the Hover API.
type APIError struct {
	// StatusCode is the HTTP status code
	StatusCode int
	// ErrorCode is the the JSON error_code field
	ErrorCode string
	// ErrorMsg is the the JSON error_message field
	ErrorMsg string
}

// Error allows APIError to match the error interface type
func (ae *APIError) Error() string {
	return fmt.Sprintf("Hover API returned error code %#v: %s", ae.ErrorCode, ae.ErrorMsg)
}

// Domains gets the list of domains (sans DNS records), billing, and other user
// information for the logged-in user
func (c *Client) Domains(ctx context.Context) ([]*Domain, error) {
	dr := &struct {
		hoverResp
		Domains []*Domain `json:"domains"`
	}{}
	code, err := c.do(ctx, dr, "GET", defaultURL+"/domains", nil)
	if err != nil {
		return nil, err
	}
	if !dr.Succeeded {
		return nil, &APIError{StatusCode: code, ErrorCode: dr.ErrorCode, ErrorMsg: dr.Error}
	}
	return dr.Domains, nil
}

// DNS gets the list of DNS domain information
func (c *Client) DNS(ctx context.Context) ([]*DNSDomain, error) {
	dr := &dnsDomainsResp{}
	code, err := c.do(ctx, dr, "GET", defaultURL+"/dns", nil)
	if err != nil {
		return nil, err
	}
	if !dr.Succeeded {
		return nil, &APIError{StatusCode: code, ErrorCode: dr.ErrorCode, ErrorMsg: dr.Error}
	}
	return dr.DNSDomains, nil
}

// GetDomain returns a single Domain's information
func (c *Client) GetDomain(ctx context.Context, domainID DomainID) (*Domain, error) {
	if domainID == "" {
		return nil, errors.New("empty domainID")
	}
	dr := &struct {
		hoverResp
		Domain *Domain `json:"domain"`
	}{}
	code, err := c.do(ctx, dr, "GET", fmt.Sprintf("%s/domains/%s", defaultURL, string(domainID)), nil)
	if err != nil {
		return nil, err
	}

	if !dr.Succeeded {
		return nil, &APIError{StatusCode: code, ErrorCode: dr.ErrorCode, ErrorMsg: dr.Error}
	}
	return dr.Domain, nil
}

type dnsDomainsResp struct {
	hoverResp
	DNSDomains []*DNSDomain `json:"domains"`
}

// GetDNSDomains returns the DNSDomains of a single domain
func (c *Client) GetDNSDomains(ctx context.Context, domainID DomainID) ([]*DNSDomain, error) {
	if domainID == "" {
		return nil, errors.New("empty domainID")
	}
	dr := &dnsDomainsResp{}
	code, err := c.do(ctx, dr, "GET", fmt.Sprintf("%s/domains/%s/dns", defaultURL, string(domainID)), nil)
	if err != nil {
		return nil, err
	}
	if !dr.Succeeded {
		return nil, &APIError{StatusCode: code, ErrorCode: dr.ErrorCode, ErrorMsg: dr.Error}
	}
	return dr.DNSDomains, nil
}

// NewDNSRecord is the data needed to create a new DNS record
type NewDNSRecord struct {
	Type    RecordType
	Name    string
	Content string
	TTL     time.Duration
}

// AddDNSRecord adds a new DNS record to a domain. Multiple calls with the same
// data will make multiple records.
func (c *Client) AddDNSRecord(ctx context.Context, domainID DomainID, rec *NewDNSRecord) error {
	if domainID == "" {
		return errors.New("empty domainID")
	}
	if rec.Content == "" {
		return errors.New("Content can't be empty")
	}
	v := url.Values{}
	v.Set("type", string(rec.Type))
	v.Set("name", rec.Name)
	v.Set("content", rec.Content)
	v.Set("ttl", strconv.Itoa(int(rec.TTL/time.Second)))
	dr := &hoverResp{}
	code, err := c.do(ctx, dr, "POST", fmt.Sprintf("%s/domains/%s/dns?%s", defaultURL, domainID, v.Encode()), nil)
	if err != nil {
		return err
	}
	if !dr.Succeeded {
		return &APIError{StatusCode: code, ErrorCode: dr.ErrorCode, ErrorMsg: dr.Error}
	}
	return nil
}

// DeleteDNSRecord deletes the DNS record specifiedy by the given DNSRecordID
func (c *Client) DeleteDNSRecord(ctx context.Context, dnsID DNSRecordID) error {
	dr := &hoverResp{}
	code, err := c.do(ctx, dr, "DELETE", fmt.Sprintf("%s/dns/%s", defaultURL, string(dnsID)), nil)
	if err != nil {
		return err
	}
	if !dr.Succeeded {
		return &APIError{StatusCode: code, ErrorCode: dr.ErrorCode, ErrorMsg: dr.Error}
	}
	return nil
}

// do performs a HTTP request given the data, unmarshals the returned JSON into
// obj, and returns the HTTP status code of the response and any errors, if any,
// that occur along the way.
func (c *Client) do(ctx context.Context, obj interface{}, method, urlStr string, r io.Reader) (int, error) {
	req, err := http.NewRequest(method, urlStr, r)
	if err != nil {
		return -1, err
	}
	req.AddCookie(c.cook)
	resp, err := ctxhttp.Do(ctx, c.hc, req)
	if err != nil {
		return -1, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return -1, err
	}
	return resp.StatusCode, json.Unmarshal(body, obj)
}

package fetch

import (
	"crypto/tls"
	"io"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"time"

	"github.com/hashicorp/go-cleanhttp"
	"github.com/hashicorp/go-retryablehttp"

	"github.com/metatube-community/metatube-sdk-go/common/random"
	"github.com/metatube-community/metatube-sdk-go/errors"
)

var DefaultFetcher = Default(&Config{RandomUserAgent: true})

type Config struct {
	// Set User-Agent Header.
	UserAgent string

	// Set Referer Header.
	Referer string

	// Enable cookies.
	EnableCookies bool

	// Use random User-Agent.
	RandomUserAgent bool

	// Return error when status is not OK.
	RaiseForStatus bool

	// HTTP Request timeout.
	Timeout time.Duration

	// Custom HTTP Transport.
	Transport http.RoundTripper

	// Skip TLS verification. Applies only
	// to *http.Transport based transport.
	SkipVerify bool
}

type Fetcher struct {
	client *http.Client
	config *Config
}

func New(c *http.Client, cfg *Config) *Fetcher {
	if cfg.RandomUserAgent {
		// assign a random user-agent.
		cfg.UserAgent = random.UserAgent()
	}
	if cfg.EnableCookies {
		jar, _ := cookiejar.New(nil)
		c.Jar = jar // assign a cookie jar.
	}
	return &Fetcher{
		client: c,
		config: cfg,
	}
}

func Default(cfg *Config) *Fetcher {
	if cfg == nil /* init if nil */ {
		cfg = new(Config)
	}
	// Enable status check by default.
	cfg.RaiseForStatus = true
	// Enable random UA if not set.
	if cfg.UserAgent == "" {
		cfg.RandomUserAgent = true
	}
	c := &retryablehttp.Client{
		HTTPClient:   cleanhttp.DefaultPooledClient(),
		RetryWaitMin: 1 * time.Second,
		RetryWaitMax: 3 * time.Second,
		RetryMax:     3,
		CheckRetry:   retryablehttp.DefaultRetryPolicy,
		Backoff:      retryablehttp.DefaultBackoff,
	}
	if cfg.Timeout > time.Second {
		c.HTTPClient.Timeout = cfg.Timeout
	}
	if cfg.Transport != nil {
		c.HTTPClient.Transport = cfg.Transport
	}
	if cfg.SkipVerify {
		if transport, ok := c.HTTPClient.Transport.(*http.Transport); ok {
			if transport.TLSClientConfig == nil {
				// init TLS config if is nil.
				transport.TLSClientConfig = &tls.Config{}
			}
			transport.TLSClientConfig.InsecureSkipVerify = true
		}
	}
	return New(c.StandardClient(), cfg)
}

func (f *Fetcher) Fetch(url string) (resp *http.Response, err error) {
	return f.Get(url)
}

func (f *Fetcher) Get(url string, opts ...Option) (resp *http.Response, err error) {
	return f.Request(http.MethodGet, url, nil, opts...)
}

func (f *Fetcher) Post(url string, body io.Reader, opts ...Option) (resp *http.Response, err error) {
	return f.Request(http.MethodPost, url, body, opts...)
}

func (f *Fetcher) Request(method, url string, body io.Reader, opts ...Option) (resp *http.Response, err error) {
	var req *http.Request
	if req, err = http.NewRequest(method, strings.TrimSpace(url), body); err != nil {
		return
	}
	c := &Context{
		req:    req,
		Config: *f.config, /* clone */
	}
	// compose options.
	var options []Option
	if c.UserAgent != "" {
		options = append(options, WithUserAgent(c.UserAgent))
	}
	if c.Referer != "" {
		options = append(options, WithReferer(c.Referer))
	}
	// apply options.
	for _, option := range append(options, opts...) {
		option.apply(c)
	}
	// make HTTP request.
	if resp, err = f.client.Do(req); err != nil {
		return
	}
	if c.RaiseForStatus && resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return nil, errors.FromCode(resp.StatusCode)
	}
	return
}

func Fetch(url string) (*http.Response, error) {
	return DefaultFetcher.Fetch(url)
}

func Get(url string, opts ...Option) (*http.Response, error) {
	return DefaultFetcher.Get(url, opts...)
}

func Post(url string, body io.Reader, opts ...Option) (*http.Response, error) {
	return DefaultFetcher.Post(url, body, opts...)
}

func Request(method, url string, body io.Reader, opts ...Option) (*http.Response, error) {
	return DefaultFetcher.Request(method, url, body, opts...)
}

var (
	_ = Fetch
	_ = Get
	_ = Post
	_ = Request
)

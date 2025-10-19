package network_test

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"

	"github.com/dewadaru/mtgo/essentials"
	"github.com/dewadaru/mtgo/network"
	"github.com/mccutchen/go-httpbin/v2/httpbin"
	"github.com/stretchr/testify/mock"
	"github.com/txthinking/socks5"
)

type DialerMock struct {
	mock.Mock
}

func (d *DialerMock) Dial(network, address string) (essentials.Conn, error) {
	args := d.Called(network, address)

	return args.Get(0).(essentials.Conn), args.Error(1) //nolint: wrapcheck, forcetypeassert
}

func (d *DialerMock) DialContext(ctx context.Context, network, address string) (essentials.Conn, error) {
	args := d.Called(ctx, network, address)

	return args.Get(0).(essentials.Conn), args.Error(1) //nolint: wrapcheck, forcetypeassert
}

type HTTPServerTestSuite struct {
	httpServer *httptest.Server
}

func (suite *HTTPServerTestSuite) SetupSuite() {
	// Initialize the HTTPBin server
	httpBin := httpbin.New()

	// Start the test server
	suite.httpServer = httptest.NewServer(httpBin.Handler())
}

func (suite *HTTPServerTestSuite) TearDownSuite() {
	suite.httpServer.Close()
}

func (suite *HTTPServerTestSuite) HTTPServerAddress() string {
	return strings.TrimPrefix(suite.httpServer.URL, "http://")
}

func (suite *HTTPServerTestSuite) MakeURL(path string) string {
	return suite.httpServer.URL + path
}

func (suite *HTTPServerTestSuite) MakeHTTPClient(dialer network.Dialer) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
				return dialer.DialContext(ctx, network, address) //nolint: wrapcheck
			},
		},
	}
}

type Socks5ServerTestSuite struct {
	socks5Listener net.Listener
	socks5Server   *socks5.Server
}

func (suite *Socks5ServerTestSuite) SetupSuite() {
	suite.socks5Listener, _ = net.Listen("tcp", "127.0.0.1:0")

	// Create server with username/password authentication using txthinking/socks5
	// Timeouts are in seconds (int), not time.Duration
	suite.socks5Server, _ = socks5.NewClassicServer(
		suite.socks5Listener.Addr().String(),
		"127.0.0.1",
		"user",
		"password",
		60, // TCP timeout in seconds
		60, // UDP timeout in seconds
	)

	// Pass nil to use default handler (standard SOCKS5 behavior)
	go suite.socks5Server.ListenAndServe(nil) //nolint: errcheck
}

func (suite *Socks5ServerTestSuite) TearDownSuite() {
	suite.socks5Listener.Close()
}

func (suite *Socks5ServerTestSuite) MakeSocks5URL(user, password string) *url.URL {
	return &url.URL{
		Scheme: "socks5",
		User:   url.UserPassword(user, password),
		Host:   suite.socks5Listener.Addr().String(),
	}
}

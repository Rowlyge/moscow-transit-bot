package telegram

import (
"context"
"fmt"
"net"
"net/http"

"golang.org/x/net/proxy"
)

// NewHTTPClient returns an *http.Client that routes all requests through
// the given SOCKS5 proxy address ("host:port"). Telegram's Bot API is
// blocked by some ISPs, so bot traffic is tunneled through a proxy
// (e.g. an SSH -D tunnel to a foreign VPS) rather than connecting directly.
func NewHTTPClient(socks5Addr string) (*http.Client, error) {
dialer, err := proxy.SOCKS5("tcp", socks5Addr, nil, proxy.Direct)
if err != nil {
return nil, fmt.Errorf("creating SOCKS5 dialer: %w", err)
}

contextDialer, ok := dialer.(proxy.ContextDialer)
if !ok {
return nil, fmt.Errorf("SOCKS5 dialer does not support context")
}

transport := &http.Transport{
DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
return contextDialer.DialContext(ctx, network, addr)
},
}

return &http.Client{Transport: transport}, nil
}

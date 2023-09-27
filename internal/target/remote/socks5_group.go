package remote

import (
	"context"
	"fmt"
	"net"

	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/module"
	"golang.org/x/net/proxy"
)

type Socks5Group struct {
	instName string

	host string
	port int

	user     string
	password string
}

func NewSocks5Group(_, instName string, _, _ []string) (module.Module, error) {
	return &Socks5Group{
		instName: instName,
	}, nil
}

func (sg *Socks5Group) Init(cfg *config.Map) error {
	cfg.String("host", true, false, "127.0.0.1", &sg.host)
	cfg.Int("port", true, false, 1080, &sg.port)

	cfg.String("user", true, false, "", &sg.user)
	cfg.String("password", true, false, "", &sg.password)

	return nil
}

func (sg *Socks5Group) Name() string {
	return "socks5"
}

func (sg *Socks5Group) InstanceName() string {
	return sg.instName
}

func (sg *Socks5Group) dialer(forward proxy.Dialer) (proxy.Dialer, error) {
	var auth *proxy.Auth
	if sg.user != "" && sg.password != "" {
		auth = &proxy.Auth{
			User:     sg.user,
			Password: sg.password,
		}
	}
	return proxy.SOCKS5("tcp", fmt.Sprintf("%s:%d", sg.host, sg.port), auth, forward)
}

func (sg *Socks5Group) Dialer(dialerFunc DialerFunc) (DialerFunc, error) {
	d := dialer{F: dialerFunc}

	socksDialer, err := sg.dialer(d)
	if err != nil {
		return nil, err
	}

	if sd, ok := socksDialer.(proxy.ContextDialer); ok {
		return sd.DialContext, nil
	}

	return nil, fmt.Errorf("socks5 dialer does not implement proxy.ContextDialer")

	//return func(ctx context.Context, network string, addr string) (net.Conn, error) {
	//	return socksDialer.Dial(network, addr)
	//}, nil
}

type dialer struct {
	F DialerFunc
}

func (d dialer) DialContext(ctx context.Context, network string, addr string) (net.Conn, error) {
	return d.F(ctx, network, addr)
}

func (d dialer) Dial(network string, addr string) (net.Conn, error) {
	return d.DialContext(context.Background(), network, addr)
}

func init() {
	module.Register("socks5", NewSocks5Group)
}

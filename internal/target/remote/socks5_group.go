package remote

import (
	"context"
	"fmt"
	"net"
	"strconv"

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
	for _, child := range cfg.Block.Children {
		switch scope := child.Name; scope {
		case "host":
			if len(child.Args) != 1 {
				return config.NodeErr(child, "exactly one argument is required")
			}
			sg.host = child.Args[0]
		case "port":
			if len(child.Args) != 1 {
				return config.NodeErr(child, "exactly one argument is required")
			}
			port, err := strconv.Atoi(child.Args[0])
			if err != nil {
				return config.NodeErr(child, "invalid port number: %v", err)
			}
			sg.port = port
		case "user":
			if len(child.Args) != 1 {
				return config.NodeErr(child, "exactly one argument is required")
			}
			sg.user = child.Args[0]
		case "password":
			if len(child.Args) != 1 {
				return config.NodeErr(child, "exactly one argument is required")
			}
			sg.password = child.Args[0]
		}
	}
	
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

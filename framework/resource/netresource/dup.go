package netresource

import "net"

func dupTCPListener(l *net.TCPListener) (*net.TCPListener, error) {
	f, err := l.File()
	if err != nil {
		return nil, err
	}
	l2, err := net.FileListener(f)
	if err != nil {
		return nil, err
	}
	return l2.(*net.TCPListener), nil
}

func dupUnixListener(l *net.UnixListener) (*net.UnixListener, error) {
	f, err := l.File()
	if err != nil {
		return nil, err
	}
	l2, err := net.FileListener(f)
	if err != nil {
		return nil, err
	}
	return l2.(*net.UnixListener), nil
}

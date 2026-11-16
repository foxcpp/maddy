//go:build unix

package tests

import (
	"fmt"
	"math/rand"
	"net"
	"os/exec"
	"strings"

	"github.com/foxcpp/maddy/framework/resource/netresource"
)

func (t *T) allocateNamedTCPPort(name string) uint16 {
	t.Helper()

	if strings.Contains(name, ":") {
		panic("cannot allocate named port with a colon in name")
	}

	if NoFDSharing || !t.allowFDSharing {
		t.Log("FD sharing disabled, using randomly generated port")
		return uint16(rand.Int31n(45536) + 20000)
	}

	l, err := net.ListenTCP("tcp", &net.TCPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func(l *net.TCPListener) {
		err := l.Close()
		if err != nil {
			t.Log("named listener close:", err)
		}
	}(l)

	f, err := l.File()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := f.Close(); err != nil {
			t.Log("named tcp close:", err)
		}
	})

	t.extraFD = append(t.extraFD, f)
	t.fdNames = append(t.fdNames, name)

	port := l.Addr().(*net.TCPAddr).Port
	t.Logf("allocated TCP port %d for name %s", l.Addr().(*net.TCPAddr).Port, name)

	return uint16(port)
}

func (t *T) addNamedTCPEnv(cmd *exec.Cmd) {
	if NoFDSharing || !t.allowFDSharing {
		for name, port := range t.ports {
			cmd.Env = append(cmd.Env,
				fmt.Sprintf("TEST_ENDPOINT_%s=tcp://127.0.0.1:%d", name, port),
				fmt.Sprintf("TEST_TLS_ENDPOINT_%s=tls://127.0.0.1:%d", name, port),
			)
		}
	}

	cmd.ExtraFiles = append(cmd.ExtraFiles, t.extraFD...)
	cmd.Env = append(cmd.Env,
		netresource.EnvListenPIDAny+"=1",
		netresource.EnvListenFDName+"="+strings.Join(t.fdNames, ":"),
	)
	for _, name := range t.fdNames {
		cmd.Env = append(cmd.Env,
			fmt.Sprintf("TEST_ENDPOINT_%s=fdname://%s", name, name),
			fmt.Sprintf("TEST_TLS_ENDPOINT_%s=fdname+tls://%s", name, name),
		)
	}
}

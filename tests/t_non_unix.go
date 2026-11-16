//go:build !unix

package tests

import "fmt"

func (t *T) allocateNamedTCPPort(name string) uint16 {
	return uint16(rand.Int31n(45536) + 20000)
}

func (t *T) addNamedTCPEnv(cmd *exec.Cmd) {
	for name, port := range t.ports {
		cmd.Env = append(cmd.Env,
			fmt.Sprintf("TEST_ENDPOINT_%s=tcp://127.0.0.1:%d", name, port),
			fmt.Sprintf("TEST_TLS_ENDPOINT_%s=tls://127.0.0.1:%d", name, port),
		)
	}
}

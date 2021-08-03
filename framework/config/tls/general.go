/*
Maddy Mail Server - Composable all-in-one email server.
Copyright © 2019-2020 Max Mazurov <fox.cpp@disroot.org>, Maddy Mail Server contributors

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

package tls

import (
	"crypto/tls"

	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/log"
)

var strVersionsMap = map[string]uint16{
	"tls1.0": tls.VersionTLS10,
	"tls1.1": tls.VersionTLS11,
	"tls1.2": tls.VersionTLS12,
	"tls1.3": tls.VersionTLS13,
	"":       0, // use crypto/tls defaults if value is not specified
}

var strCiphersMap = map[string]uint16{
	// TLS 1.0 - 1.2 cipher suites.
	"RSA-WITH-RC4128-SHA":                tls.TLS_RSA_WITH_RC4_128_SHA,
	"RSA-WITH-3DES-EDE-CBC-SHA":          tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA,
	"RSA-WITH-AES128-CBC-SHA":            tls.TLS_RSA_WITH_AES_128_CBC_SHA,
	"RSA-WITH-AES256-CBC-SHA":            tls.TLS_RSA_WITH_AES_256_CBC_SHA,
	"RSA-WITH-AES128-CBC-SHA256":         tls.TLS_RSA_WITH_AES_128_CBC_SHA256,
	"RSA-WITH-AES128-GCM-SHA256":         tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
	"RSA-WITH-AES256-GCM-SHA384":         tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
	"ECDHE-ECDSA-WITH-RC4128-SHA":        tls.TLS_ECDHE_ECDSA_WITH_RC4_128_SHA,
	"ECDHE-ECDSA-WITH-AES128-CBC-SHA":    tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
	"ECDHE-ECDSA-WITH-AES256-CBC-SHA":    tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
	"ECDHE-RSA-WITH-RC4128-SHA":          tls.TLS_ECDHE_RSA_WITH_RC4_128_SHA,
	"ECDHE-RSA-WITH-3DES-EDE-CBC-SHA":    tls.TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA,
	"ECDHE-RSA-WITH-AES128-CBC-SHA":      tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
	"ECDHE-RSA-WITH-AES256-CBC-SHA":      tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
	"ECDHE-ECDSA-WITH-AES128-CBC-SHA256": tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256,
	"ECDHE-RSA-WITH-AES128-CBC-SHA256":   tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256,
	"ECDHE-RSA-WITH-AES128-GCM-SHA256":   tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
	"ECDHE-ECDSA-WITH-AES128-GCM-SHA256": tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
	"ECDHE-RSA-WITH-AES256-GCM-SHA384":   tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
	"ECDHE-ECDSA-WITH-AES256-GCM-SHA384": tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
	"ECDHE-RSA-WITH-CHACHA20-POLY1305":   tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
	"ECDHE-ECDSA-WITH-CHACHA20-POLY1305": tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
}

var strCurvesMap = map[string]tls.CurveID{
	"p256":   tls.CurveP256,
	"p384":   tls.CurveP384,
	"p521":   tls.CurveP521,
	"X25519": tls.X25519,
}

// TLSversionsDirective parses directive with arguments that specify
// minimum and maximum supported TLS versions.
//
// It returns [2]uint16 value for use in corresponding fields from tls.Config.
func TLSVersionsDirective(_ *config.Map, node config.Node) (interface{}, error) {
	switch len(node.Args) {
	case 1:
		value, ok := strVersionsMap[node.Args[0]]
		if !ok {
			return nil, config.NodeErr(node, "invalid TLS version value: %s", node.Args[0])
		}
		return [2]uint16{value, value}, nil
	case 2:
		minValue, ok := strVersionsMap[node.Args[0]]
		if !ok {
			return nil, config.NodeErr(node, "invalid TLS version value: %s", node.Args[0])
		}
		maxValue, ok := strVersionsMap[node.Args[1]]
		if !ok {
			return nil, config.NodeErr(node, "invalid TLS version value: %s", node.Args[1])
		}
		return [2]uint16{minValue, maxValue}, nil
	default:
		return nil, config.NodeErr(node, "expected 1 or 2 arguments")
	}
}

// TLSCiphersDirective parses directive with arguments that specify
// list of ciphers to offer to clients (or to use for outgoing connections).
//
// It returns list of []uint16 with corresponding cipher IDs.
func TLSCiphersDirective(_ *config.Map, node config.Node) (interface{}, error) {
	if len(node.Args) == 0 {
		return nil, config.NodeErr(node, "expected at least 1 argument, got 0")
	}

	res := make([]uint16, 0, len(node.Args))
	for _, arg := range node.Args {
		cipherId, ok := strCiphersMap[arg]
		if !ok {
			return nil, config.NodeErr(node, "unknown cipher: %s", arg)
		}
		res = append(res, cipherId)
	}
	log.Debugln("tls: using non-default cipherset:", node.Args)
	return res, nil
}

// TLSCurvesDirective parses directive with arguments that specify
// elliptic curves to use during TLS key exchange.
//
// It returns []tls.CurveID.
func TLSCurvesDirective(_ *config.Map, node config.Node) (interface{}, error) {
	if len(node.Args) == 0 {
		return nil, config.NodeErr(node, "expected at least 1 argument, got 0")
	}

	res := make([]tls.CurveID, 0, len(node.Args))
	for _, arg := range node.Args {
		curveId, ok := strCurvesMap[arg]
		if !ok {
			return nil, config.NodeErr(node, "unknown curve: %s", arg)
		}
		res = append(res, curveId)
	}
	log.Debugln("tls: using non-default curve preferences:", node.Args)
	return res, nil
}

package module

import "crypto/tls"

// TLSLoader interface is module interface that can be used to supply TLS
// certificates to TLS-enabled endpoints.
//
// The interface is intentionally kept simple, all configuration and parameters
// necessary are to be provided using conventional module configuration.
//
// If loader returns multiple certificate chains - endpoint will serve them
// based on SNI matching.
//
// Note that loading function will be called for each connections - it is
// highly recommended to cache parsed form.
//
// Modules implementing this interface should be registered with prefix
// "tls.loader." in name.
type TLSLoader interface {
	LoadCerts() ([]tls.Certificate, error)
}

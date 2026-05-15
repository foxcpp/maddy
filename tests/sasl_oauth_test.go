//go:build integration

package tests_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"
	"github.com/foxcpp/maddy/tests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSMTPOAuthSASL(tt *testing.T) {
	tt.Parallel()

	var introspectionResult map[string]any
	var introspectionStatusCode int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(tt, http.MethodGet, r.Method)
		assert.Equal(tt, "/introspect", r.URL.String())
		assert.Equal(tt, "Bearer testtoken", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(introspectionStatusCode)

		err := json.NewEncoder(w).Encode(introspectionResult)
		require.NoError(tt, err)
	}))
	tt.Cleanup(srv.Close)

	t := tests.NewT(tt)
	t.DNS(nil)
	smtpPort := t.Port("smtp")
	t.Config(`

smtp tcp://127.0.0.1:{env:TEST_PORT_smtp} {
	tls off
	insecure_auth
	hostname mx.maddy.test
	auth oauth {
		introspection auth
		introspection_url ` + srv.URL + `/introspect
	}

	# Use authorize_sender to quickly check effective auth username via MAIL FROM.
	defer_sender_reject
	check {
		authorize_sender
	}

	deliver_to dummy
}
`)
	t.Run(1)
	defer t.Close()

	introspectionResult = map[string]any{
		"email": "testuser@maddy.test",
	}
	introspectionStatusCode = http.StatusOK

	client, err := smtp.Dial("127.0.0.1:" + strconv.Itoa(int(smtpPort)))
	require.NoError(t, err)
	defer client.Close()

	require.NoError(t, client.Hello("localhost"))
	require.True(t, client.SupportsAuth(sasl.OAuthBearer))
	require.NoError(t, client.Auth(sasl.NewOAuthBearerClient(&sasl.OAuthBearerOptions{
		Username: "",
		Token:    "testtoken",
	})))
	require.NoError(t, client.Mail("testuser@maddy.test", nil))
}

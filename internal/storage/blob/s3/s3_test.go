package s3

import (
	"net/http/httptest"
	"testing"

	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/internal/storage/blob"
	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
)

func TestFS(t *testing.T) {
	var (
		backend gofakes3.Backend
		faker   *gofakes3.GoFakeS3
		ts      *httptest.Server
	)

	blob.TestStore(t, func() module.BlobStore {
		backend = s3mem.New()
		faker = gofakes3.New(backend)
		ts = httptest.NewServer(faker.Server())

		if err := backend.CreateBucket("maddy-test"); err != nil {
			panic(err)
		}

		st := &Store{instName: "test"}
		err := st.Init(config.NewMap(map[string]interface{}{}, config.Node{
			Children: []config.Node{
				{
					Name: "endpoint",
					Args: []string{ts.Listener.Addr().String()},
				},
				{
					Name: "secure",
					Args: []string{"false"},
				},
				{
					Name: "access_key",
					Args: []string{"access-key"},
				},
				{
					Name: "secret_key",
					Args: []string{"secret-key"},
				},
				{
					Name: "bucket",
					Args: []string{"maddy-test"},
				},
			},
		}))
		if err != nil {
			panic(err)
		}

		return st
	}, func(store module.BlobStore) {
		ts.Close()

		backend = s3mem.New()
		faker = gofakes3.New(backend)
		ts = httptest.NewServer(faker.Server())
	})

	if ts != nil {
		ts.Close()
	}
}

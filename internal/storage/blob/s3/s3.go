package s3

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/log"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

const modName = "storage.blob.s3"

const (
	credsTypeFileMinio = "file_minio"
	credsTypeFileAWS   = "file_aws"
	credsTypeAccessKey = "access_key"
	credsTypeIAM       = "iam"
	credsTypeDefault   = credsTypeAccessKey
)

type Store struct {
	instName string
	log      log.Logger

	endpoint string
	cl       *minio.Client

	bucketName   string
	objectPrefix string
}

func New(_, instName string, _, inlineArgs []string) (module.Module, error) {
	if len(inlineArgs) != 0 {
		return nil, fmt.Errorf("%s: expected 0 arguments", modName)
	}

	return &Store{
		instName: instName,
		log:      log.Logger{Name: modName},
	}, nil
}

func (s *Store) Init(cfg *config.Map) error {
	var (
		secure          bool
		accessKeyID     string
		secretAccessKey string
		credsType       string
		location        string
	)
	cfg.String("endpoint", false, true, "", &s.endpoint)
	cfg.Bool("secure", false, true, &secure)
	cfg.String("access_key", false, true, "", &accessKeyID)
	cfg.String("secret_key", false, true, "", &secretAccessKey)
	cfg.String("bucket", false, true, "", &s.bucketName)
	cfg.String("region", false, false, "", &location)
	cfg.String("object_prefix", false, false, "", &s.objectPrefix)
	cfg.String("creds", false, false, credsTypeDefault, &credsType)

	if _, err := cfg.Process(); err != nil {
		return err
	}
	if s.endpoint == "" {
		return fmt.Errorf("%s: endpoint not set", modName)
	}

	var creds *credentials.Credentials

	switch credsType {
	case credsTypeFileMinio:
		creds = credentials.NewFileMinioClient("", "")
	case credsTypeFileAWS:
		creds = credentials.NewFileAWSCredentials("", "")
	case credsTypeIAM:
		creds = credentials.NewIAM("")
	case credsTypeAccessKey:
		creds = credentials.NewStaticV4(accessKeyID, secretAccessKey, "")
	default:
		creds = credentials.NewStaticV4(accessKeyID, secretAccessKey, "")
	}

	cl, err := minio.New(s.endpoint, &minio.Options{
		Creds:  creds,
		Secure: secure,
		Region: location,
	})
	if err != nil {
		return fmt.Errorf("%s: %w", modName, err)
	}

	s.cl = cl
	return nil
}

func (s *Store) Name() string {
	return modName
}

func (s *Store) InstanceName() string {
	return s.instName
}

type s3blob struct {
	pw      *io.PipeWriter
	didSync bool
	errCh   chan error
}

func (b *s3blob) Sync() error {
	// We do this in Sync instead of Close because
	// backend may not actually check the error of Close.
	// The problematic restriction is that Sync can now be called
	// only once.
	if b.didSync {
		panic("storage.blob.s3: Sync called twice for a blob object")
	}

	b.pw.Close()
	b.didSync = true
	return <-b.errCh
}

func (b *s3blob) Write(p []byte) (n int, err error) {
	return b.pw.Write(p)
}

func (b *s3blob) Close() error {
	if !b.didSync {
		if err := b.pw.CloseWithError(fmt.Errorf("storage.blob.s3: blob closed without Sync")); err != nil {
			panic(err)
		}
	}
	return nil
}

func (s *Store) Create(ctx context.Context, key string, blobSize int64) (module.Blob, error) {
	pr, pw := io.Pipe()
	errCh := make(chan error, 1)

	go func() {
		partSize := uint64(0)
		if blobSize == module.UnknownBlobSize {
			// Without this, minio-go will allocate 500 MiB buffer which
			// is a little too much.
			// https://github.com/minio/minio-go/issues/1478
			partSize = 1 * 1024 * 1024 /* 1 MiB */
		}
		_, err := s.cl.PutObject(ctx, s.bucketName, s.objectPrefix+key, pr, blobSize, minio.PutObjectOptions{
			PartSize: partSize,
		})
		if err != nil {
			if err := pr.CloseWithError(fmt.Errorf("s3 PutObject: %w", err)); err != nil {
				panic(err)
			}
		}
		errCh <- err
	}()

	return &s3blob{
		pw:    pw,
		errCh: errCh,
	}, nil
}

func (s *Store) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	obj, err := s.cl.GetObject(ctx, s.bucketName, s.objectPrefix+key, minio.GetObjectOptions{})
	if err != nil {
		resp := minio.ToErrorResponse(err)
		if resp.StatusCode == http.StatusNotFound {
			return nil, module.ErrNoSuchBlob
		}
		return nil, err
	}
	return obj, nil
}

func (s *Store) Delete(ctx context.Context, keys []string) error {
	var lastErr error
	for _, k := range keys {
		lastErr = s.cl.RemoveObject(ctx, s.bucketName, s.objectPrefix+k, minio.RemoveObjectOptions{})
		if lastErr != nil {
			s.log.Error("failed to delete object", lastErr, s.objectPrefix+k)
		}
	}
	return lastErr
}

func init() {
	var _ module.BlobStore = &Store{}
	module.Register(modName, New)
}

package s3provider

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/prxssh/csv-ingestor/ingest-service/config"
	"github.com/prxssh/csv-ingestor/ingest-service/internal/storage"
	"go.opentelemetry.io/contrib/instrumentation/github.com/aws/aws-sdk-go-v2/otelaws"
)

type Provider struct {
	client  *s3.Client
	presign *s3.PresignClient
	bucket  string
}

func New(ctx context.Context) (*Provider, error) {
	opts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(config.Env.S3Region),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(
				config.Env.S3AccessKeyID,
				config.Env.S3SecretAccessKey,
				"",
			),
		),
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("s3: load config: %w", err)
	}
	otelaws.AppendMiddlewares(&cfg.APIOptions)

	clientOpts := []func(*s3.Options){}
	if config.Env.S3Endpoint != "" {
		clientOpts = append(clientOpts,
			func(o *s3.Options) {
				o.BaseEndpoint = aws.String(config.Env.S3Endpoint)
				o.UsePathStyle = true
			},
		)
	}

	client := s3.NewFromConfig(cfg, clientOpts...)
	return &Provider{
		client:  client,
		presign: s3.NewPresignClient(client),
		bucket:  config.Env.S3Bucket,
	}, nil
}

func (p *Provider) InitMultipartUpload(
	ctx context.Context,
	key, contentType string,
) (*storage.InitResult, error) {
	out, err := p.client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket:      aws.String(p.bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return nil, fmt.Errorf("s3: create multipart upload: %w", err)
	}

	return &storage.InitResult{
		UploadID: aws.ToString(out.UploadId),
		Key:      key,
	}, nil
}

func (p *Provider) PresignPartUpload(
	ctx context.Context,
	key, uploadID string,
	partNumber int32,
	expires time.Duration,
) (*storage.PresignedPart, error) {
	req, err := p.presign.PresignUploadPart(ctx, &s3.UploadPartInput{
		Bucket:     aws.String(p.bucket),
		Key:        aws.String(key),
		UploadId:   aws.String(uploadID),
		PartNumber: aws.Int32(partNumber),
	}, s3.WithPresignExpires(expires))
	if err != nil {
		return nil, fmt.Errorf("s3: presign part %d: %w", partNumber, err)
	}

	return &storage.PresignedPart{
		PartNumber: partNumber,
		URL:        req.URL,
	}, nil
}

func (p *Provider) CompleteMultipartUpload(
	ctx context.Context,
	key, uploadID string,
	parts []storage.CompletedPart,
) (string, error) {
	completedParts := make([]types.CompletedPart, len(parts))
	for i, pt := range parts {
		completedParts[i] = types.CompletedPart{
			PartNumber: aws.Int32(pt.PartNumber),
			ETag:       aws.String(pt.ETag),
		}
	}

	out, err := p.client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(p.bucket),
		Key:      aws.String(key),
		UploadId: aws.String(uploadID),
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: completedParts,
		},
	})
	if err != nil {
		return "", fmt.Errorf("s3: complete multipart upload: %w", err)
	}

	return aws.ToString(out.Location), nil
}

func (p *Provider) AbortMultipartUpload(ctx context.Context, key, uploadID string) error {
	_, err := p.client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
		Bucket:   aws.String(p.bucket),
		Key:      aws.String(key),
		UploadId: aws.String(uploadID),
	})
	if err != nil {
		return fmt.Errorf("s3: abort multipart upload: %w", err)
	}
	return nil
}

func (p *Provider) GetObjectStream(ctx context.Context, key string) (io.ReadCloser, error) {
	out, err := p.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(p.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("s3: get object stream: %w", err)
	}

	return out.Body, nil
}

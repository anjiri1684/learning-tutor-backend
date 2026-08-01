package services

import (
	"context"
	"fmt"
	"io"
	"sync"

	config "github.com/anjiri1684/language_tutor/configs"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var (
	r2Client   *s3.Client
	r2ClientMu sync.Mutex
)

func getR2Client() (*s3.Client, error) {
	r2ClientMu.Lock()
	defer r2ClientMu.Unlock()

	if r2Client != nil {
		return r2Client, nil
	}

	accountID := config.Config("R2_ACCOUNT_ID")
	accessKeyID := config.Config("R2_ACCESS_KEY_ID")
	secretAccessKey := config.Config("R2_SECRET_ACCESS_KEY")

	endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com", accountID)

	cfg, err := awsConfig.LoadDefaultConfig(context.Background(),
		awsConfig.WithRegion("auto"),
		awsConfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, "")),
	)
	if err != nil {
		return nil, err
	}

	r2Client = s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
	})

	return r2Client, nil
}

func UploadObject(key string, body io.Reader, contentType string) error {
	client, err := getR2Client()
	if err != nil {
		return err
	}

	bucket := config.Config("R2_BUCKET_NAME")

	_, err = client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        body,
		ContentType: aws.String(contentType),
	})
	return err
}

func GetObjectStream(key string) (io.ReadCloser, string, error) {
	client, err := getR2Client()
	if err != nil {
		return nil, "", err
	}

	bucket := config.Config("R2_BUCKET_NAME")

	output, err := client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, "", err
	}

	contentType := "application/octet-stream"
	if output.ContentType != nil {
		contentType = *output.ContentType
	}

	return output.Body, contentType, nil
}

func DeleteObject(key string) error {
	client, err := getR2Client()
	if err != nil {
		return err
	}

	bucket := config.Config("R2_BUCKET_NAME")

	_, err = client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	return err
}

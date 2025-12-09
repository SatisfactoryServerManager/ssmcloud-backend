package repositories

import (
	"context"
	"fmt"
	"mime"
	"os"
	"path/filepath"

	ssmtypes "github.com/SatisfactoryServerManager/ssmcloud-backend/internal/types"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	s3manager "github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var (
	S3Client   *s3.Client
	bucketName string
)

func GetS3Client() (*s3.Client, error) {
	if S3Client != nil {
		return S3Client, nil
	}

	endpoint := os.Getenv("STORAGE_S3_ENDPOINT")
	accessKey := os.Getenv("STORAGE_S3_ACCESSKEYID")
	secretKey := os.Getenv("STORAGE_S3_SECRETKEY")
	region := os.Getenv("STORAGE_S3_REGION")

	if endpoint == "" {
		return nil, fmt.Errorf("STORAGE_S3_ENDPOINT is not set")
	}

	cfg, err := config.LoadDefaultConfig(
		context.Background(),
		config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
		),
		config.WithBaseEndpoint(endpoint),
	)
	if err != nil {
		return nil, err
	}

	S3Client = s3.NewFromConfig(cfg, func(o *s3.Options) {
		// Required for Garage, MinIO, Ceph, etc.
		o.UsePathStyle = false
		o.Region = region
	})

	return S3Client, nil
}

func CreateSSMBucket() error {
	bucketName = os.Getenv("STORAGE_S3_BUCKET")
	client, err := GetS3Client()
	if err != nil {
		return err
	}

	_, headErr := client.HeadBucket(context.Background(), &s3.HeadBucketInput{
		Bucket: aws.String(bucketName),
	})
	if headErr == nil {
		return nil // exists
	}

	// Try to create bucket
	_, err = client.CreateBucket(context.Background(), &s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	})

	if err != nil {
		return err
	}

	return nil
}

func getMimeTypeByExtension(file string) string {
	ext := filepath.Ext(file)
	if ext == ".log" {
		return "text/plain"
	}
	return mime.TypeByExtension(ext)
}

func UploadAgentFile(fileIdentity ssmtypes.StorageFileIdentity, objectPath string) (string, error) {
	client, err := GetS3Client()
	if err != nil {
		return "", err
	}

	file, err := os.Open(fileIdentity.LocalFilePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return "", err
	}

	size := stat.Size()

	uploader := s3manager.NewUploader(client)

	_, err = uploader.Upload(context.Background(), &s3.PutObjectInput{
		Bucket:        aws.String(bucketName),
		Key:           aws.String(objectPath),
		Body:          file,
		ContentType:   aws.String(getMimeTypeByExtension(fileIdentity.FileName)),
		ContentLength: &size,
	})

	if err != nil {
		return "", err
	}

	// Build object URL
	endpoint := os.Getenv("STORAGE_S3_ENDPOINT")
	objectURL := fmt.Sprintf("%s/%s/%s", endpoint, bucketName, objectPath)

	_ = os.Remove(fileIdentity.LocalFilePath)

	return objectURL, nil
}

func GetAgentFile(objectPath string) (*s3.GetObjectOutput, error) {
	client, err := GetS3Client()
	if err != nil {
		return nil, err
	}

	bucket := os.Getenv("STORAGE_S3_BUCKET")

	resp, err := client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(objectPath),
	})

	if err != nil {
		return nil, err
	}

	return resp, nil
}

func HasAgentFile(objectPath string) bool {
	client, err := GetS3Client()
	if err != nil {
		fmt.Println(err.Error())
		return false
	}

	_, err = client.HeadObject(context.Background(), &s3.HeadObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectPath),
	})

	return err == nil
}

func DeleteAccountFolder(accountId string) error {
	if accountId == "" {
		return nil
	}

	client, err := GetS3Client()
	if err != nil {
		return err
	}

	ctx := context.Background()
	prefix := accountId + "/"

	// List objects
	resp, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucketName),
		Prefix: aws.String(prefix),
	})
	if err != nil {
		return err
	}

	// Delete each object
	for _, obj := range resp.Contents {
		_, err := client.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(bucketName),
			Key:    obj.Key,
		})
		if err != nil {
			return err
		}

		fmt.Println("Deleted:", *obj.Key)
	}

	return nil
}

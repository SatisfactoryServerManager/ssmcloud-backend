package repositories

import (
	"context"
	"fmt"
	"os"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/types"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/utils/config"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var (
	MinioClient *minio.Client
	bucketName  = "ssm"
)

func GetMinioClient() (*minio.Client, error) {
	if MinioClient == nil {

		configData, err := config.GetConfigData()
		if err != nil {
			return nil, err
		}

		endpoint := configData.Storage.Minio.Endpoint
		accessKeyID := configData.Storage.Minio.AccessKeyId
		secretAccessKey := configData.Storage.Minio.SecretKey
		useSSL := configData.Storage.Minio.UseSSL

		minioClient, err := minio.New(endpoint, &minio.Options{
			Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
			Secure: useSSL,
		})
		if err != nil {
			return nil, err
		}

		MinioClient = minioClient
	}

	return MinioClient, nil
}

func CreateSSMBucket() error {
	minioClient, err := GetMinioClient()

	if err != nil {
		return err
	}

	err = minioClient.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{})
	if err != nil {
		// Check if the bucket already exists
		exists, errBucketExists := minioClient.BucketExists(context.Background(), bucketName)
		if errBucketExists == nil && exists {
			return nil
		} else {
			return err
		}
	}
	return nil
}

func UploadAgentFile(fileIdentity types.StorageFileIdentity, objectPath string) (string, error) {

	minioClient, err := GetMinioClient()

	if err != nil {
		return "", err
	}

	file, err := os.Open(fileIdentity.LocalFilePath)
	if err != nil {
		return "", err
	}

	defer file.Close()

	fileStat, err := file.Stat()
	if err != nil {
		return "", err
	}

	_, err = minioClient.PutObject(
		context.Background(),
		bucketName,
		objectPath,
		file,
		fileStat.Size(),
		minio.PutObjectOptions{ContentType: "text/plain"},
	)

	if err != nil {
		return "", err
	}

	objectURL := fmt.Sprintf("%s/%s/%s", minioClient.EndpointURL().String(), bucketName, objectPath)

	if err := os.Remove(fileIdentity.LocalFilePath); err != nil {
		return "", err
	}

	return objectURL, nil
}

func GetAgentFile(objectPath string) (*minio.Object, error) {
	minioClient, err := GetMinioClient()

	if err != nil {
		return nil, err
	}

	object, err := minioClient.GetObject(context.Background(), bucketName, objectPath, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}

	return object, nil
}

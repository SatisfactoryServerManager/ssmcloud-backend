package services

import (
	"mime/multipart"
	"path/filepath"
	"strings"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/utils"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/utils/config"
)

type StorageFileIdentity struct {
	UUID          string
	FileName      string
	Extension     string
	LocalFilePath string
}

func InitStorageService() {
	utils.CreateFolder(filepath.Join(config.DataDir, "temp"))
	utils.CreateFolder(filepath.Join(config.DataDir, "account_data"))
}

func ConvertUploadToFileIdentity(file *multipart.FileHeader) StorageFileIdentity {
	uuid := utils.RandStringBytes(16)

	normizliedFileName := filepath.Base(strings.ReplaceAll(file.Filename, "\\", "/"))

	extension := filepath.Ext(normizliedFileName)
	newFileName := uuid + "_" + normizliedFileName

	tempDir := filepath.Join(config.DataDir, "temp")

	destFilePath := filepath.Join(tempDir, newFileName)

	return StorageFileIdentity{
		UUID:          uuid,
		FileName:      normizliedFileName,
		Extension:     extension,
		LocalFilePath: destFilePath,
	}
}

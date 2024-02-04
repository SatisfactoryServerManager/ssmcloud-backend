package services

import (
	"mime/multipart"
	"path/filepath"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/utils"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/utils/config"
)

type StorageFileIdentity struct {
	UUID          string
	FileName      string
	Extension     string
	LocalFilePath string
}

func ConvertUploadToFileIdentity(file *multipart.FileHeader) StorageFileIdentity {
	uuid := utils.RandStringBytes(16)

	extension := filepath.Ext(file.Filename)
	newFileName := uuid + "_" + file.Filename

	tempDir := filepath.Join(config.DataDir, "temp")
	utils.CreateFolder(tempDir)

	destFilePath := filepath.Join(tempDir, newFileName)

	return StorageFileIdentity{
		UUID:          uuid,
		FileName:      file.Filename,
		Extension:     extension,
		LocalFilePath: destFilePath,
	}
}

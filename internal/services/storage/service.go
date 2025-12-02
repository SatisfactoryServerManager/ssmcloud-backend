package storage

import (
	"mime/multipart"
	"path/filepath"
	"strings"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/types"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/utils"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/utils/config"
)

func InitStorageService() {
	utils.CreateFolder(filepath.Join(config.DataDir, "temp"))
}

func ConvertUploadToFileIdentity(file *multipart.FileHeader) types.StorageFileIdentity {
	uuid := utils.RandStringBytes(16)

	normizliedFileName := filepath.Base(strings.ReplaceAll(file.Filename, "\\", "/"))

	extension := filepath.Ext(normizliedFileName)
	newFileName := uuid + "_" + normizliedFileName

	tempDir := filepath.Join(config.DataDir, "temp")

	destFilePath := filepath.Join(tempDir, newFileName)

	return types.StorageFileIdentity{
		UUID:          uuid,
		FileName:      normizliedFileName,
		Extension:     extension,
		LocalFilePath: destFilePath,
		Filesize:      file.Size,
	}
}

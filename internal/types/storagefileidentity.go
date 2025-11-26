package types

type StorageFileIdentity struct {
	UUID          string
	FileName      string
	Extension     string
	LocalFilePath string
	Filesize      int64
}

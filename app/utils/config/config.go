package config

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/utils"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/utils/logger"
	"github.com/joho/godotenv"
)

var (
	_config *Config
	DataDir string
)

type Config struct {
	ConfigBaseDir  string
	ConfigFileName string
	ConfigFilePath string
	Loaded         bool
	ConfigData     ConfigData
}

type ConfigData struct {
	Version  string `json:"version"`
	HTTPBind string `json:"http_bind"`
	Database struct {
		Host string `json:"host"`
		Port int    `json:"port"`
		DB   string `json:"database"`
		User string `json:"username"`
		Pass string `json:"password"`
	} `json:"db"`
	Storage struct {
		Minio struct {
			Endpoint    string `json:"endpoint"`
			AccessKeyId string `json:"accessKeyId"`
			SecretKey   string `json:"secretKey"`
			UseSSL      bool   `json:"useSSL"`
		} `json:"minio"`
	} `json:"storage"`
	Flags struct {
		DisablePurgeAccountData bool `json:"disablePurgeAccountData"`
		UseMinioStorage         bool `json:"useMinioStorage"`
	}
}

func (config *Config) LoadConfigData() error {
	basePath := filepath.Dir(config.ConfigFilePath)

	if err := utils.CreateFolder(basePath); err != nil {
		utils.CheckError(err)
	}

	if !utils.CheckFileExists(config.ConfigFilePath) {
		//new config file
		file, err := os.Create(config.ConfigFilePath)
		if err != nil {
			return err
		}
		file.Close()

		config.Loaded = true

		config.SetDefaultValues()
		if err := config.SaveConfigData(); err != nil {
			return err
		}
		return nil
	}

	f, err := os.Open(config.ConfigFilePath)
	if err != nil {
		return err
	}
	defer f.Close()

	byteValue, _ := io.ReadAll(f)

	if err := json.Unmarshal(byteValue, &config.ConfigData); err != nil {
		return err
	}

	config.Loaded = true
	config.SetDefaultValues()
	if err := config.SaveConfigData(); err != nil {
		return err
	}
	return nil
}

func (config *Config) SetDefaultValues() {

	godotenv.Load(".env.local")

	config.ConfigData.Version = "v1.0.61"

	config.ConfigData.Database.Host = os.Getenv("DB_HOST")
	config.ConfigData.Database.DB = os.Getenv("DB_DB")
	config.ConfigData.Database.Port, _ = strconv.Atoi(os.Getenv("DB_PORT"))
	config.ConfigData.Database.User = os.Getenv("DB_USER")
	config.ConfigData.Database.Pass = os.Getenv("DB_PASS")

	if os.Getenv("HOST_PORT") != "" {
		config.ConfigData.HTTPBind = os.Getenv("HOST_PORT")
	} else {
		config.ConfigData.HTTPBind = ":3000"
	}

	config.ConfigData.Flags.DisablePurgeAccountData = os.Getenv("FLAG_DISABLEPURGEACCOUNTDATA") == "true"
	config.ConfigData.Flags.UseMinioStorage = os.Getenv("FLAG_USEMINIOSTORAGE") == "true"

    if config.ConfigData.Flags.UseMinioStorage {
        config.ConfigData.Storage.Minio.Endpoint = os.Getenv("STORAGE_MINIO_ENDPOINT")
        config.ConfigData.Storage.Minio.AccessKeyId = os.Getenv("STORAGE_MINIO_ACCESSKEYID")
        config.ConfigData.Storage.Minio.SecretKey = os.Getenv("STORAGE_MINIO_SECRETKEY")
        config.ConfigData.Storage.Minio.UseSSL = os.Getenv("STORAGE_MINIO_USESSL") == "true"
    }
}

func (config *Config) SaveConfigData() error {
	data, err := GetConfigData()

	if err != nil {
		return err
	}

	file, _ := json.MarshalIndent(data, "", "    ")

	if err := os.WriteFile(config.ConfigFilePath, file, 0755); err != nil {
		return err
	}
	return nil
}

func InitConfig() error {

	HomeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	DataDir = filepath.Join(HomeDir, "ssmcloud_data")

	_config = &Config{}
	_config.ConfigBaseDir = filepath.Join(DataDir, "config")
	_config.ConfigFileName = "SSM.config.json"
	_config.ConfigFilePath = filepath.Join(_config.ConfigBaseDir, _config.ConfigFileName)

	if err := _config.LoadConfigData(); err != nil {
		return err
	}

	logDir := filepath.Join(DataDir, "logs")
	logger.SetupLoggers("SSM", logDir)

	logger.GetInfoLogger().Printf("Config Location: %s", GetConfig().ConfigFilePath)
	return nil
}

func GetConfig() *Config {
	if _config == nil {
		_config = &Config{}
	}

	return _config
}

func GetConfigData() (*ConfigData, error) {
	if GetConfig() == nil {
		return nil, fmt.Errorf("error getting config data, config is nil")
	}

	if !GetConfig().Loaded {
		return nil, fmt.Errorf("error getting config data, config is not loaded")
	}

	return &GetConfig().ConfigData, nil
}

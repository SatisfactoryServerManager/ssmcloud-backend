package utils

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func CheckError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func CreateFolder(folderPath string) error {
	if _, err := os.Stat(folderPath); errors.Is(err, os.ErrNotExist) {
		err := os.MkdirAll(folderPath, os.ModePerm)
		if err != nil {
			return err
		}
	}
	return nil
}

func CheckFileExists(filepath string) bool {
	_, err := os.Stat(filepath)
	return !os.IsNotExist(err)
}

func RandStringBytes(n int) string {
	letterBytes := "abcdefghijklmnopqrstuvwxyz1234567890"
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

func TwoFASecretGenerator(n int) string {
	letterBytes := "abcdefghijklmnopqrstuvwxyz234567"
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

func MoveFile(sourcePath, destPath string) error {
	inputFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("Couldn't open source file: %s", err)
	}
	outputFile, err := os.Create(destPath)
	if err != nil {
		inputFile.Close()
		return fmt.Errorf("Couldn't open dest file: %s", err)
	}
	defer outputFile.Close()
	_, err = io.Copy(outputFile, inputFile)
	inputFile.Close()
	if err != nil {
		return fmt.Errorf("Writing to output file failed: %s", err)
	}
	// The copy was successful, so now delete the original file
	err = os.Remove(sourcePath)
	if err != nil {
		return fmt.Errorf("Failed removing original file: %s", err)
	}
	return nil
}

func ReadLastNBtyesFromFile(fname string, nbytes int64) (string, error) {
	file, err := os.Open(fname)
	if err != nil {
		return "", err
	}
	defer file.Close()

	stat, statErr := file.Stat()
	if statErr != nil {
		return "", statErr
	}

	if stat.Size() < nbytes {
		nbytes = stat.Size()
	}

	buf := make([]byte, nbytes)

	start := stat.Size() - nbytes
	_, err = file.ReadAt(buf, start)

	if err != nil {
		return "", err
	}

	return string(buf), nil

}

func TrackTime(start time.Time, name string) {
	elapsed := time.Since(start)
	fmt.Printf("%s took %s\n", name, elapsed)
}

func GetAPIKeyFromContext(ctx context.Context) (*string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Errorf(codes.Unauthenticated, "missing metadata")
	}

	apiKeys := md["x-api-key"]
	if len(apiKeys) == 0 {
		return nil, status.Errorf(codes.Unauthenticated, "missing API key")
	}

	apiKey := &apiKeys[0]

	return apiKey, nil
}

func StructToPBStruct(a interface{}, b interface{}) {
	bytes, _ := json.Marshal(a)
	json.Unmarshal(bytes, b)
}

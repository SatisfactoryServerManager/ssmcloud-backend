package main

import (
	"fmt"
	"path/filepath"
	"strings"
)

func main() {

	fileSrc := "D:\\Test\\Test.sav"

	fmt.Println(fileSrc)
	fmt.Println(filepath.Base(fileSrc))
	convertedSrc := strings.ReplaceAll(fileSrc, "\\", "/")
	fmt.Println(convertedSrc)
	fmt.Println(filepath.Base(convertedSrc))

}

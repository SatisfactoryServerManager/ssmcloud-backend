package main

import (
	"fmt"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/utils"
)

func main() {

	fileContents, err := utils.ReadLastNBtyesFromFile("./test.go", 500)

	if err != nil {
		panic(err)
	}

	fmt.Println(fileContents)
}

package main

import (
	"flag"
	"fmt"
)

func main() {

	var modePtr string
	flag.StringVar(&modePtr, "mode", "i", "choosing program mode")

	flag.Parse()

	switch modePtr {
	case "z":
		files := getFileList("./mydir")

		metas, archive, err := zipFiles(files)
		if err != nil {
			fmt.Printf("Error has occured: %s", err.Error())
		}

		err = createSzp("data.szp", "my.key", "my.crt", metas, archive)
		if err != nil {
			fmt.Printf("Error has occured: %s", err.Error())
		}
	case "x":

	case "i":

	default:
		fmt.Println("Use -mode only with \"z\", \"x\", \"i\" flags")
	}

}

//Receives name of directory to zip and returns list of files to zip
func getFileList(dir string) []string {

	return nil
}

func zipFiles(files []string) (metas, archive []byte, err error) {

	return nil, nil, nil
}

func createSzp(newFile, keyPath, certPath string, meta, archive []byte) error {

	return nil
}

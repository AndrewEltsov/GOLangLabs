package main

import (
	"flag"
	"fmt"
	"io/ioutil"
)

func main() {

	var modePtr string
	flag.StringVar(&modePtr, "mode", "z", "choosing program mode")

	flag.Parse()

	switch modePtr {
	case "z":
		files, err := getFileList("./mydir")
		if err != nil {
			fmt.Printf("Error has occured: %s", err.Error())
		}
		fmt.Println(files)

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
func getFileList(dir string) ([]string, error) {
	var fileNames []string
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if file.IsDir() {
			subdirfiles, err := getFileList(dir + "/" + file.Name())
			if err != nil {
				return nil, err
			}

			fileNames = append(fileNames, subdirfiles...)
		} else {
			fileNames = append(fileNames, dir+"/"+file.Name())
		}
	}

	return fileNames, nil
}

func zipFiles(files []string) (metas, archive []byte, err error) {

	return nil, nil, nil
}

func createSzp(newFile, keyPath, certPath string, meta, archive []byte) error {

	return nil
}

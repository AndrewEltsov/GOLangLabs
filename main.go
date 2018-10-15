package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha1"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"flag"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"log"
	"os"
	"time"

	"github.com/fullsailor/pkcs7"
	yaml "gopkg.in/yaml.v2"
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

		err = createSzp(files, "data.szp", "my.key", "my.crt")
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

func zipFiles(files []string) (archive []byte, err error) {
	buf := new(bytes.Buffer)

	w := zip.NewWriter(buf)

	for _, file := range files {
		fileBytes, err := ioutil.ReadFile(file)
		if err != nil {

			return nil, err
		}

		f, err := w.Create(file[1:])
		if err != nil {
			return nil, err
		}
		_, err = f.Write(fileBytes)
		if err != nil {
			return nil, err
		}
	}

	err = w.Close()
	if err != nil {
		return nil, err
	}

	//ioutil.WriteFile("bufer", buf.Bytes(), 0666)
	return buf.Bytes(), nil
}

func createSzp(files []string, newFile, keyPath, certPath string) error {
	metas, err := createMeta(files)
	if err != nil {
		return err
	}

	archive, err := zipFiles(files)
	if err != nil {
		return err
	}

	metaLength := make([]byte, 4)
	binary.LittleEndian.PutUint32(metaLength, uint32(len(metas)))

	data := append(metaLength, metas...)
	data = append(data, archive...)

	szp, err := signFile(data, keyPath, certPath)
	if err != nil {
		return err
	}

	file, err := os.Create(newFile)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.Write(szp)
	if err != nil {
		return err
	}

	return nil
}

func createMeta(files []string) (metaBytes []byte, err error) {
	metas := make(map[string]meta)

	var metaInfo meta
	var h hash.Hash

	for i := range files {
		openedFile, err := os.Open(files[i])
		if err != nil {
			return nil, err
		}
		info, err := openedFile.Stat()
		if err != nil {

			return nil, err
		}

		metaInfo.Name = info.Name()
		metaInfo.Size = info.Size()
		metaInfo.ModTime = info.ModTime()

		h = sha1.New()
		if _, err := io.Copy(h, openedFile); err != nil {
			return nil, err
		}

		metaInfo.Hash = string(h.Sum(nil))

		metas[files[i][1:]] = metaInfo

		err = openedFile.Close()
		if err != nil {
			return nil, err
		}
	}

	bytesToZip, err := yaml.Marshal(metas)
	if err != nil {
		return nil, err
	}

	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)

	f, err := w.Create("metas")
	if err != nil {
		return nil, err
	}
	_, err = f.Write(bytesToZip)
	if err != nil {
		return nil, err
	}

	err = w.Close()
	if err != nil {
		return nil, err
	}

	//ioutil.WriteFile("bufer", buf.Bytes(), 0666)
	return buf.Bytes(), nil
}

func signFile(fileBytes []byte, keyPath, certPath string) ([]byte, error) {
	signedData, err := pkcs7.NewSignedData(fileBytes)
	if err != nil {
		fmt.Printf("Cannot initialize signed data: %s", err)
	}

	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		log.Fatal(err)
	}

	parsedCert, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return nil, err
	}

	if err := signedData.AddSigner(parsedCert, cert.PrivateKey, pkcs7.SignerInfoConfig{}); err != nil {
		return nil, err
	}

	signedFileBytes, err := signedData.Finish()
	if err != nil {
		return nil, err
	}

	return signedFileBytes, nil
}

type meta struct {
	Name    string
	Size    int64
	ModTime time.Time
	Hash    string
}

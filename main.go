package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha1"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/fullsailor/pkcs7"
	yaml "gopkg.in/yaml.v2"
)

func main() {

	var modePtr, hashPtr string
	flag.StringVar(&modePtr, "mode", "z", "choosing program mode")
	flag.StringVar(&hashPtr, "hash", "e594936d61b2c2857613ed81d88ba24acd153f7c", "fingerprint of certificate")

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
		err := extractSzp("data.szp", hashPtr)
		if err != nil {
			fmt.Printf("Error has occured: %s", err.Error())
		}
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

//returns bytes of new zip archive
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

	return buf.Bytes(), nil
}

//creates signed zip package
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

//creates zipped metadata file
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

		metaInfo.Hash = fmt.Sprintf("%x", h.Sum(nil))

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

	return buf.Bytes(), nil
}

//signes file which consists of metadata and archive
func signFile(fileBytes []byte, keyPath, certPath string) ([]byte, error) {
	signedData, err := pkcs7.NewSignedData(fileBytes)
	if err != nil {
		return nil, err
	}

	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, err
	}

	parsedCert, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return nil, err
	}

	//h := sha1.New()
	fmt.Printf("SHA1 fingerprint is %x\n", sha1.Sum(parsedCert.Raw))

	if err := signedData.AddSigner(parsedCert, cert.PrivateKey, pkcs7.SignerInfoConfig{}); err != nil {
		return nil, err
	}

	signedFileBytes, err := signedData.Finish()
	if err != nil {
		return nil, err
	}

	return signedFileBytes, nil
}

//extracts signed zip package
func extractSzp(filePath, fingerprint string) error {
	fileBytes, err := verifySzp(filePath, fingerprint)
	if err != nil {
		return err
	}

	sizeBytes := fileBytes[0:4]
	metaSize := binary.LittleEndian.Uint32(sizeBytes)

	var metasArchive []byte
	metasArchive = fileBytes[4 : 4+metaSize]

	metas, err := unzipMeta(metasArchive, metaSize)
	if err != nil {
		return err
	}

	err = unzipArchive(fileBytes[4+metaSize:], metas, "Temp")
	if err != nil {
		return err
	}

	return nil
}

//verifies certificate of signed zip package
func verifySzp(filePath, ShaCert string) (content []byte, err error) {
	fileBytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	p7, err := pkcs7.Parse(fileBytes)
	if err != nil {
		return nil, err
	}

	err = p7.Verify()
	if err != nil {
		return nil, err
	}

	if ShaCert != fmt.Sprintf("%x", sha1.Sum(p7.Certificates[0].Raw)) {
		fmt.Println(ShaCert)
		fmt.Printf("%x\n", sha1.Sum(p7.Certificates[0].Raw))
		return nil, errors.New("error has occured: invalid sha1")
	}
	fmt.Println("Certificate is correct")

	return p7.Content, nil
}

//unzips metadate from zip archive
func unzipMeta(archiveBytes []byte, size uint32) (map[string]meta, error) {
	metas := make(map[string]meta)

	r, err := zip.NewReader(bytes.NewReader(archiveBytes), int64(size))
	if err != nil {
		return nil, err
	}

	rc, err := r.File[0].Open()
	if err != nil {
		return nil, err
	}

	buf := make([]byte, r.File[0].UncompressedSize)

	rc.Read(buf)

	err = yaml.Unmarshal(buf, &metas)
	if err != nil {
		return nil, err
	}
	return metas, nil
}

//unzips zip archive with data
func unzipArchive(archiveBytes []byte, metas map[string]meta, dest string) error {
	r, err := zip.NewReader(bytes.NewReader(archiveBytes), int64(len(archiveBytes)))
	if err != nil {
		return err
	}
	isValid, err := checkSha(r.File, metas)
	if err != nil {
		return err
	}
	if !isValid {
		return errors.New("Files are invalid")
	}

	for _, zf := range r.File {
		fpath := filepath.Join(dest, zf.Name)

		err = os.MkdirAll(filepath.Dir(fpath), os.ModePerm)
		if err != nil {
			return err
		}

		dst, err := os.Create(fpath)
		if err != nil {
			return err
		}
		defer dst.Close()
		src, err := zf.Open()
		if err != nil {
			return err
		}
		defer src.Close()
		_, err = io.Copy(dst, src)
		if err != nil {
			return err
		}
	}

	return nil
}

//validates files of archive
func checkSha(files []*zip.File, metas map[string]meta) (bool, error) {
	for _, zf := range files {
		rc, err := zf.Open()
		if err != nil {
			return false, err
		}

		buf := make([]byte, zf.UncompressedSize)

		rc.Read(buf)

		if metas[zf.Name].Hash != fmt.Sprintf("%x", sha1.Sum(buf)) {
			return false, nil
		}
	}
	fmt.Println("Files are valid")
	return true, nil
}

//struct to marshall information about files
type meta struct {
	Name    string
	Size    int64
	ModTime time.Time
	Hash    string
}

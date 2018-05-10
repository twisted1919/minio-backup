package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/mholt/archiver"
	"github.com/minio/minio-go"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Constants
const (
	configFileName = "minio-backup-config.json"
)

// Main configuration strct
type configuration struct {
	Endpoint        string `json:"endpoint"`
	AccessKeyID     string `json:"access-key-id"`
	SecretAccessKey string `json:"secret-access-key"`
	BucketName      string `json:"bucket-name"`
	UseSSL          bool   `json:"ssl"`
	Location        string `json:"location"`
	MaxBackups      int    `json:"max-backups"`
	BackupPrefix    string `json:"backup-prefix"`
	BackupFolder    string `json:"backup-folder"`
}

// Helper for loading the configuration from file
func (c *configuration) loadFromJSONFile(configFile string) {

	// Paths where to look for config
	// The home directory one has priority
	paths := []string{
		fmt.Sprintf("~/.%s", configFile),
	}

	if currentPath, err := filepath.Abs(filepath.Dir(os.Args[0])); err == nil {
		paths = append(paths, currentPath+string(os.PathSeparator)+configFile)
	}

	configFilePath := ""
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			configFilePath = path
			break
		}
	}

	if configFilePath == "" {
		return
	}

	b, err := ioutil.ReadFile(configFilePath)
	if err != nil {
		log.Fatalf("Configuration file read error: %s", err)
	}

	err = json.Unmarshal(b, c)
	if err != nil {
		log.Fatalf("Configuration file marshal error: %s", err)
	}
}

// Helper to create a new config object
func newConfiguration() *configuration {
	return &configuration{
		UseSSL:       true,
		Location:     "us-east-1",
		MaxBackups:   5,
		BackupPrefix: "backup-",
	}
}

func main() {

	// Use default config as default in parsed values from flags
	defaultConfig := newConfiguration()
	defaultConfig.loadFromJSONFile(configFileName)

	// Define the variables we will use, with default on ENV variables
	endpoint := flag.String("endpoint", defaultConfig.Endpoint, "the endpoint")
	accessKeyID := flag.String("access-key-id", defaultConfig.AccessKeyID, "the access key id")
	secretAccessKey := flag.String("secret-access-key", defaultConfig.SecretAccessKey, "the secret access key")
	bucketName := flag.String("bucket-name", defaultConfig.BucketName, "the bucket name")
	useSSL := flag.Bool("ssl", defaultConfig.UseSSL, "whether to use ssl")
	location := flag.String("location", defaultConfig.Location, "the location name")
	maxBackups := flag.Int("max-backups", defaultConfig.MaxBackups, "maximum number of backups to keep")
	backupPrefix := flag.String("backup-prefix", defaultConfig.BackupPrefix, "backup prefix")
	backupFolder := flag.String("backup-folder", defaultConfig.BackupFolder, "the folder to backup")

	flag.Parse()

	config := &configuration{
		Endpoint:        *endpoint,
		AccessKeyID:     *accessKeyID,
		SecretAccessKey: *secretAccessKey,
		BucketName:      *bucketName,
		UseSSL:          *useSSL,
		Location:        *location,
		MaxBackups:      *maxBackups,
		BackupPrefix:    *backupPrefix,
		BackupFolder:    *backupFolder,
	}

	// Some basic checks before anything else
	if len(strings.TrimSpace(config.Endpoint)) == 0 {
		log.Fatalln("Please specify an endpoint: --endpoint=...")
	}

	if len(strings.TrimSpace(config.AccessKeyID)) == 0 {
		log.Fatalln("Please specify a access-key-id: --access-key-id=...")
	}

	if len(strings.TrimSpace(config.SecretAccessKey)) == 0 {
		log.Fatalln("Please specify a secret-access-key: --secret-access-key=...")
	}

	if len(strings.TrimSpace(config.BucketName)) == 0 {
		log.Fatalln("Please specify a bucket-name: --bucket-name=...")
	}

	if len(strings.TrimSpace(config.BackupFolder)) == 0 {
		log.Fatalln("Please specify a backup-folder: --backup-folder=...")
	}

	// if the backup folder does not exist
	if _, err := os.Stat(config.BackupFolder); os.IsNotExist(err) {
		log.Fatalln(fmt.Sprintf("The folder %s does not exist!", config.BackupFolder))
	}

	// Initialize minio client object.
	minioClient, err := minio.New(config.Endpoint, config.AccessKeyID, config.SecretAccessKey, config.UseSSL)
	if err != nil {
		log.Fatalln(err)
	}

	// Create the bucket if it does not exists
	err = minioClient.MakeBucket(config.BucketName, config.Location)
	if err != nil {
		// Check to see if we already own this bucket (which happens if you run this twice)
		exists, err := minioClient.BucketExists(config.BucketName)
		if err == nil && exists {
			log.Printf("We already own %s\n", config.BucketName)
		} else {
			log.Fatalln(err)
		}
	}
	log.Printf("Using bucket: %s\n", config.BucketName)

	// List all objects from a bucket-name with a matching prefix.
	doneCh := make(chan struct{})
	defer close(doneCh)

	// Populate a slice of minio.ObjectInfo
	var objects []minio.ObjectInfo
	for object := range minioClient.ListObjectsV2(config.BucketName, config.BackupPrefix, true, doneCh) {
		if object.Err != nil {
			fmt.Println(object.Err)
			continue
		}
		objects = append(objects, object)
	}

	// Make sure we only keep latest X backups
	if len(objects) >= *maxBackups {
		objects = objects[len(objects)-3 : 3]
		for _, object := range objects {
			err = minioClient.RemoveObject(config.BucketName, object.Key)
			if err != nil {
				log.Fatalln(err)
			}
			log.Println(fmt.Sprintf("Successfully removed object: %s", object.Key))
		}
	}

	// Create the backup archive locally, in /tmp
	archiveName := fmt.Sprintf("%s%s.zip", config.BackupPrefix, time.Now().Format("2006-01-02.15-04-05"))
	tmpFilePath := fmt.Sprintf("/tmp/%s", archiveName)

	log.Println(fmt.Sprintf("Creating: %s which will contain the contents of: %s", tmpFilePath, config.BackupFolder))

	// And make the zip
	err = archiver.Zip.Make(tmpFilePath, []string{config.BackupFolder})
	if err != nil {
		log.Fatalln(err)
	}

	// Upload the zip file with FPutObject
	n, err := minioClient.FPutObject(config.BucketName, archiveName, tmpFilePath, minio.PutObjectOptions{ContentType: "application/zip"})
	if err != nil {
		log.Fatalln(err)
	}

	err = os.Remove(tmpFilePath)
	if err != nil {
		log.Println(err)
	}

	log.Printf("Successfully uploaded %s of size %d\n", archiveName, n)
}

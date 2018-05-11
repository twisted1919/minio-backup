package main

import (
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/mholt/archiver"
	"github.com/minio/minio-go"
	gomail "gopkg.in/gomail.v2"
	"io/ioutil"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"
)

// Constants
const (
	configFileName = "minio-backup-config.json"
	resultSuccess = "success"
	resultError = "error"
	resultInfo = "info"
)

// Main configuration struct
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

	SmtpHostname  string `json:"smtp-hostname"`
	SmtpPort      int    `json:"smtp-port"`
	SmtpUsername  string `json:"smtp-username"`
	SmtpPassword  string `json:"smtp-password"`
	SmtpFromEmail string `json:"smtp-from-email"`

	NotifySuccess bool   `json:"notify-success"`
	NotifyError   bool   `json:"notify-error"`
	NotifyEmail   string `json:"notify-email"`
}

// Helper for loading the configuration from file
func (c *configuration) loadFromJSONFile(configFile string) {

	// Paths where to look for config
	var paths []string

	// The home directory has priority
	if usr, err := user.Current(); err == nil {
		paths = append(paths, fmt.Sprintf("%s%s.%s", usr.HomeDir, string(os.PathSeparator), configFile))
	}

	// The current directory
	if currentPath, err := filepath.Abs(filepath.Dir(os.Args[0])); err == nil {
		paths = append(paths, fmt.Sprintf("%s%s%s", currentPath, string(os.PathSeparator), configFile))
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
		SmtpPort:     25,
	}
}

// Structure to hold any result message
type resultMessage struct {
	messageType string
	message     string
	timestamp   string
}

func newResultMessage(messageType string, message string) resultMessage {
	return resultMessage{
		messageType: messageType,
		message:     message,
		timestamp:   time.Now().Format("2006/01/02 15:04:05"),
	}
}

// Structure for the result
type result struct {
	config   *configuration
	messages []resultMessage
}

// Helper to add a message to the store
func (r *result) message(rm resultMessage) *result {
	log.Println(rm.message)
	r.messages = append(r.messages, rm)
	return r
}

// Stop execution with error code
func (r *result) fatal() {
	os.Exit(1)
}

// Stop execution with success code
func (r *result) ok() {
	os.Exit(0)
}

// Email the results if allowed and possible
func (r *result) email() *result {

	if r.config.SmtpHostname == "" || r.config.SmtpFromEmail == "" || r.config.NotifyEmail == "" {
		return r
	}

	if len(r.messages) == 0 {
		return r
	}

	var hasError, hasSuccess bool
	for _, m := range r.messages {
		if !hasSuccess && r.config.NotifySuccess && m.messageType == resultSuccess {
			hasSuccess = true
			continue
		}
		if !hasError && r.config.NotifyError && m.messageType == resultError {
			hasError = true
			continue
		}
	}

	if !hasError && !hasSuccess {
		return r
	}

	hostname := ""
	if name, err := os.Hostname(); err == nil {
		hostname = name
	}

	subject := fmt.Sprintf("[%s]: Backup status", hostname)
	message := ""
	for _, m := range r.messages {
		message += fmt.Sprintf("%s %s: %s<br />", m.timestamp, strings.ToUpper(m.messageType), m.message)
	}
	message = strings.Trim(message, "<br />")

	m := gomail.NewMessage()
	m.SetHeader("From", r.config.SmtpFromEmail)
	m.SetHeader("To", r.config.NotifyEmail)
	m.SetHeader("Subject", subject)
	m.SetBody("text/html", message)

	d := gomail.NewDialer(r.config.SmtpHostname, r.config.SmtpPort, r.config.SmtpUsername, r.config.SmtpPassword)
	d.TLSConfig = &tls.Config{InsecureSkipVerify: true}

	if err := d.DialAndSend(m); err != nil {
		fmt.Println(err)
	}

	return r
}

// The entry point
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

	smtpHostname := flag.String("smtp-hostname", defaultConfig.SmtpHostname, "the hostname used for the smtp server")
	smtpPort := flag.Int("smtp-port", defaultConfig.SmtpPort, "the port used for the smtp server")
	smtpUsername := flag.String("smtp-username", defaultConfig.SmtpUsername, "the username used for the smtp server")
	smtpPassword := flag.String("smtp-password", defaultConfig.SmtpPassword, "the password used for the smtp server")
	smtpFromEmail := flag.String("smtp-from-email", defaultConfig.SmtpFromEmail, "the FROM email used for the smtp server")

	notifySuccess := flag.Bool("notify-success", defaultConfig.NotifySuccess, "whether to notify on success messages")
	notifyError := flag.Bool("notify-error", defaultConfig.NotifyError, "whether to notify on error messages")
	notifyEmail := flag.String("notify-email", defaultConfig.NotifyEmail, "to whom to send the email notification")

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

		SmtpHostname:    *smtpHostname,
		SmtpPort:        *smtpPort,
		SmtpUsername:    *smtpUsername,
		SmtpPassword:    *smtpPassword,
		SmtpFromEmail:   *smtpFromEmail,

		NotifySuccess:   *notifySuccess,
		NotifyError:     *notifyError,
		NotifyEmail:     *notifyEmail,
	}

	// create the result object which we will send at the end
	res := &result{
		config: config,
	}

	// Some basic checks before anything else
	if len(strings.TrimSpace(config.Endpoint)) == 0 {
		res.message(newResultMessage(resultInfo, "Please specify an endpoint: --endpoint=...")).fatal()
	}

	if len(strings.TrimSpace(config.AccessKeyID)) == 0 {
		res.message(newResultMessage(resultInfo, "Please specify a access-key-id: --access-key-id=...")).fatal()
	}

	if len(strings.TrimSpace(config.SecretAccessKey)) == 0 {
		res.message(newResultMessage(resultInfo, "Please specify a secret-access-key: --secret-access-key=...")).fatal()
	}

	if len(strings.TrimSpace(config.BucketName)) == 0 {
		res.message(newResultMessage(resultInfo, "Please specify a bucket-name: --bucket-name=...")).fatal()
	}

	if len(strings.TrimSpace(config.BackupFolder)) == 0 {
		res.message(newResultMessage(resultInfo, "Please specify a backup-folder: --backup-folder=...")).fatal()
	}

	// if the backup folder does not exist
	if _, err := os.Stat(config.BackupFolder); os.IsNotExist(err) {
		res.message(newResultMessage(resultError, fmt.Sprintf("The folder %s does not exist!", config.BackupFolder))).fatal()
	}

	res.message(newResultMessage(resultInfo, fmt.Sprintf("Starting backup for %s", config.BackupFolder)))

	// Initialize minio client object.
	minioClient, err := minio.New(config.Endpoint, config.AccessKeyID, config.SecretAccessKey, config.UseSSL)
	if err != nil {
		res.message(newResultMessage(resultError, err.Error())).email().fatal()
	}

	// Create the bucket if it does not exists
	err = minioClient.MakeBucket(config.BucketName, config.Location)
	if err != nil {
		// Check to see if we already own this bucket (which happens if you run this twice)
		exists, err := minioClient.BucketExists(config.BucketName)
		if err == nil && exists {
			res.message(newResultMessage(resultInfo, fmt.Sprintf("We already own %s", config.BucketName)))
		} else {
			res.message(newResultMessage(resultError, err.Error())).email().fatal()
		}
	}
	res.message(newResultMessage(resultInfo, fmt.Sprintf("Using bucket: %s", config.BucketName)))

	// List all objects from a bucket-name with a matching prefix.
	doneCh := make(chan struct{})
	defer close(doneCh)

	// Populate a slice of minio.ObjectInfo
	var objects []minio.ObjectInfo
	for object := range minioClient.ListObjectsV2(config.BucketName, config.BackupPrefix, true, doneCh) {
		if object.Err != nil {
			res.message(newResultMessage(resultError, object.Err.Error()))
			continue
		}
		objects = append(objects, object)
	}

	// Make sure we only keep latest X backups
	if config.MaxBackups > 0 && len(objects) > config.MaxBackups {
		// remove newer X backups from the slice and leave only the one to be deleted
		objects = objects[:len(objects)-config.MaxBackups]
		for _, object := range objects {
			err = minioClient.RemoveObject(config.BucketName, object.Key)
			if err != nil {
				res.message(newResultMessage(resultError, err.Error()))
				continue
			}
			res.message(newResultMessage(resultSuccess, fmt.Sprintf("Successfully removed remote object: %s", object.Key)))
		}
	}

	// Create the backup archive locally, in /tmp
	archiveName := fmt.Sprintf("%s%s.zip", config.BackupPrefix, time.Now().Format("2006-01-02.15-04-05"))
	tmpFilePath := fmt.Sprintf("/tmp/%s", archiveName)

	res.message(newResultMessage(resultInfo, fmt.Sprintf("Creating: %s which will contain the contents of: %s", tmpFilePath, config.BackupFolder)))

	// And make the zip
	err = archiver.Zip.Make(tmpFilePath, []string{config.BackupFolder})
	if err != nil {
		res.message(newResultMessage(resultError, err.Error())).email().fatal()
	}

	// Upload the zip file with FPutObject
	n, err := minioClient.FPutObject(config.BucketName, archiveName, tmpFilePath, minio.PutObjectOptions{ContentType: "application/zip"})
	if err != nil {
		res.message(newResultMessage(resultError, err.Error())).email().fatal()
	}

	// Upload went okay
	res.message(newResultMessage(resultSuccess, fmt.Sprintf("Successfully uploaded %s of size %d", archiveName, n)))

	// Remove created archive
	err = os.Remove(tmpFilePath)
	if err != nil {
		res.message(newResultMessage(resultError, err.Error())).email().fatal()
	}

	// Removed the object
	res.message(newResultMessage(resultSuccess, fmt.Sprintf("Successfully removed %s from local storage", archiveName)))

	// We're done, all went okay
	res.email().ok()
}

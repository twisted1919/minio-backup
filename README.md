# Minio Backup
A small tool to backup data and send it to your Minio powered server  

You will need GO installed in your environment  
Following packages are needed:  
`go get github.com/minio/minio-go`  
`go get github.com/mholt/archiver`  
`go get gopkg.in/gomail.v2`  

### RUN  
If you already have a GO environment in place, easiest way is to install it like:  
`go get github.com/twisted1919/minio-backup`  
then call it with  
`minio-backup ...`  

You can also download or clone this repository locally and then run it like:    
`go run main.go ...`  

If there is no GO environment, download one of the ready made binaries from releases page, then:  
1. `sudo mv minio-backup-linux-amd64 /usr/local/bin/minio-backup`    
2. `sudo chmod +x /usr/local/bin/minio-backup`    
4. `minio-backup ...`  

Ready made binaries are cross compiled, like:  
`env GOOS=linux GOARCH=amd64 go build -o bin/minio-backup-linux-amd64 main.go`  

### Options  
Following options are available:  

| CLI Flag  | Config file | Default | Required | Details |
| ------------- | ------------- | ------------- | ------------- | ------------- |
| endpoint  | endpoint  | none  |  yes | your minio host address  |
| access-key-id  | access-key-id  | none  | yes  | the access key id  |
| secret-access-key  | secret-access-key  | none | yes  | the secret access key  |
| bucket-name  | bucket-name  | none  | yes  | the name of the bucket where to send the backup  |
| ssl  | ssl  |  true | no  | whether to use ssl when connecting to the endpoint  |
| location  | location  | us-east-1 | no  | the zone on the endpoint  |
| max-backups  | max-backups  | 5 | no  | how many backups to keep  |
| backup-prefix  | backup-prefix  | backup- | no | the prefix for the backup files  |
| backup-folder  | backup-folder  | none | yes | the folder to backup, absolute path  |  
| smtp-hostname  | smtp-hostname  | none | no | the hostname used for the smtp server  |  
| smtp-port  | smtp-port  | 25 | no | the port used for the smtp server  |  
| smtp-username  | smtp-username  | none | no | the username used for the smtp server  |  
| smtp-password  | smtp-password  | none | no | the password used for the smtp server  |  
| smtp-from-email  | smtp-from-email  | none | no | the FROM email used for the smtp server  |  
| notify-success  | notify-success  | false | no | whether to notify on success messages  |  
| notify-error  | notify-error  | false | no | whether to notify on error messages  |  
| notify-email  | notify-email  | none | no | to whom to send the email notification  |  

You can pass above options as command line flags, i.e:
`minio-backup --endpoint="..." --access-key-id="..."`   

Alternatively, you can also create a config file in json format, 
either in your home directory or in same directory with the `minio-backup` binary:  
``` json  
{
    "endpoint": "...",
    "access-key-id": "...",
    "secret-access-key": "...",
    "...": "..."
}
```
If you put the config file in your home directory, it should be named `.minio-backup-config.json`.  
If you put the config file in same directory with the binary, it should be named `minio-backup-config.json`.  
 
Links:  
https://golang.org/doc/install  
https://www.minio.io/  
package s3client

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var S3Client *s3.Client
var BucketName string

func init() {
	sdkConfig, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Printf("unable to load SDK config, %v\n", err)
	}
	S3Client = s3.NewFromConfig(sdkConfig)

	BucketName = os.Getenv("S3_BUCKET_NAME")
	fmt.Printf("s3 bucket name is: %s\n", BucketName)

}

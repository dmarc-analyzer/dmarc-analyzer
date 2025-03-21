// Package s3client provides functionality for interacting with Amazon S3 storage.
// It handles the initialization of the S3 client and provides access to S3 operations
// for storing and retrieving DMARC report emails.
package s3client

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3Client is the global AWS S3 client instance used throughout the application.
// It provides methods for interacting with S3 buckets and objects.
var S3Client *s3.Client

// BucketName is the name of the S3 bucket where DMARC report emails are stored.
// It's read from the S3_BUCKET_NAME environment variable during initialization.
var BucketName string

// init initializes the S3 client when the package is imported.
// It loads the AWS SDK configuration using default credential providers,
// creates a new S3 client, and sets the target bucket name from environment variables.
func init() {
	// Load AWS SDK configuration from default sources (environment, shared credentials, etc.)
	sdkConfig, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Printf("unable to load SDK config, %v\n", err)
	}
	
	// Create a new S3 client using the loaded configuration
	S3Client = s3.NewFromConfig(sdkConfig)

	// Get the S3 bucket name from environment variable
	BucketName = os.Getenv("S3_BUCKET_NAME")
	fmt.Printf("s3 bucket name is: %s\n", BucketName)
}

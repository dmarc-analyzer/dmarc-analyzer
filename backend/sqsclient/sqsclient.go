// Package sqsclient provides functionality for interacting with Amazon SQS.
// It handles the initialization of the SQS client and provides access to SQS operations
// for receiving and processing messages from the queue.
package sqsclient

import (
	"context"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

// SQSClient is the global AWS SQS client instance used throughout the application.
// It provides methods for interacting with SQS queues.
var SQSClient *sqs.Client

// QueueURL is the URL of the SQS queue where messages are received.
// It's read from the SQS_QUEUE_URL environment variable during initialization.
var QueueURL string

// init initializes the SQS client when the package is imported.
// It loads the AWS SDK configuration using default credential providers,
// creates a new SQS client, and sets the target queue URL from environment variables.
func init() {
	// Load AWS SDK configuration from default sources (environment, shared credentials, etc.)
	sdkConfig, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Printf("unable to load SDK config, %v\n", err)
	}

	// Create a new SQS client using the loaded configuration
	SQSClient = sqs.NewFromConfig(sdkConfig)

	// Get the SQS queue URL from environment variable
	QueueURL = os.Getenv("SQS_QUEUE_URL")
	log.Printf("SQS queue URL is: %s\n", QueueURL)
}

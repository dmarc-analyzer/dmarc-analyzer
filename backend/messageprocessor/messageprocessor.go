// Package messageprocessor provides functionality for processing SQS messages
// and triggering DMARC report parsing and database operations.
package messageprocessor

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/dmarc-analyzer/dmarc-analyzer/backend"
	"github.com/dmarc-analyzer/dmarc-analyzer/backend/db"
	"github.com/dmarc-analyzer/dmarc-analyzer/backend/model"
	"github.com/dmarc-analyzer/dmarc-analyzer/backend/sqsclient"
)

// S3Event represents the structure of an S3 event notification
// that gets sent to SQS when a new object is created in S3
type S3Event struct {
	Records []S3EventRecord `json:"Records"`
}

// S3EventRecord represents a single record in an S3 event
type S3EventRecord struct {
	EventName string   `json:"eventName"`
	S3        S3Entity `json:"s3"`
}

// S3Entity contains information about the S3 object that triggered the event
type S3Entity struct {
	Object S3Object `json:"object"`
}

// S3Object contains details about the S3 object
type S3Object struct {
	Key string `json:"key"`
}

// ProcessMessage processes a single SQS message containing S3 event information
// and triggers the DMARC report parsing and database insertion
func ProcessMessage(message types.Message) error {
	// Parse the message body to extract S3 event information
	var s3Event S3Event
	if err := json.Unmarshal([]byte(*message.Body), &s3Event); err != nil {
		return fmt.Errorf("failed to unmarshal S3 event: %v", err)
	}

	// Process each record in the S3 event
	for _, record := range s3Event.Records {
		// Only process ObjectCreated events
		if record.EventName != "ObjectCreated:Put" && record.EventName != "ObjectCreated:Post" {
			log.Printf("Skipping event %s for object %s", record.EventName, record.S3.Object.Key)
			continue
		}

		messageID := record.S3.Object.Key
		log.Printf("Processing new email: %s", messageID)

		// Check if this message has already been processed
		var count int64
		db.DB.Model(&model.DmarcReportEntry{}).Where("message_id = ?", messageID).Count(&count)
		if count > 0 {
			log.Printf("Message %s already processed, skipping", messageID)
			continue
		}

		// Parse the DMARC report
		feedback, err := backend.ParseNewMail(messageID)
		if err != nil {
			log.Printf("Failed to parse email %s: %v", messageID, err)
			continue
		}

		// Parse the DMARC report into database entries
		reports := backend.ParseDmarcReport(feedback, messageID)
		if len(reports) == 0 {
			log.Printf("No DMARC report entries found for message %s", messageID)
			continue
		}

		// Insert the reports into the database
		result := db.DB.Create(reports)
		if result.Error != nil {
			log.Printf("Failed to insert reports for message %s: %v", messageID, result.Error)
			continue
		}

		log.Printf("Successfully processed message %s, inserted %d report entries", messageID, len(reports))
	}

	return nil
}

// StartMessageConsumer starts a continuous message consumer that polls the SQS queue
// and processes incoming messages
func StartMessageConsumer(ctx context.Context) error {
	log.Println("Starting SQS message consumer...")

	for {
		select {
		case <-ctx.Done():
			log.Println("Message consumer stopped")
			return ctx.Err()
		default:
			// Receive messages from the queue
			input := &sqs.ReceiveMessageInput{
				QueueUrl:            aws.String(sqsclient.QueueURL),
				MaxNumberOfMessages: 10,
				WaitTimeSeconds:     20, // Long polling
				VisibilityTimeout:   30, // 30 seconds to process each message
			}

			result, err := sqsclient.SQSClient.ReceiveMessage(ctx, input)
			if err != nil {
				log.Printf("Failed to receive messages: %v", err)
				time.Sleep(5 * time.Second) // Wait before retrying
				continue
			}

			// Process each received message
			for _, message := range result.Messages {
				log.Printf("Processing message: %s", *message.MessageId)

				// Process the message
				if err := ProcessMessage(message); err != nil {
					log.Printf("Failed to process message %s: %v", *message.MessageId, err)
					// Note: In a production environment, you might want to implement
					// a dead letter queue for failed messages
					continue
				}

				// Delete the message from the queue after successful processing
				deleteInput := &sqs.DeleteMessageInput{
					QueueUrl:      aws.String(sqsclient.QueueURL),
					ReceiptHandle: message.ReceiptHandle,
				}

				if _, err := sqsclient.SQSClient.DeleteMessage(ctx, deleteInput); err != nil {
					log.Printf("Failed to delete message %s: %v", *message.MessageId, err)
				} else {
					log.Printf("Successfully deleted message: %s", *message.MessageId)
				}
			}

			// If no messages were received, continue polling
			if len(result.Messages) == 0 {
				log.Println("No messages received, continuing to poll...")
			}
		}
	}
}

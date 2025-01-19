package main

import (
	"context"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/dmarc-analyzer/dmarc-analyzer/backend"
	"github.com/dmarc-analyzer/dmarc-analyzer/backend/db"
	"github.com/dmarc-analyzer/dmarc-analyzer/backend/model"
	"github.com/dmarc-analyzer/dmarc-analyzer/backend/s3client"
)

func main() {
	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(s3client.BucketName),
	}

	paginator := s3.NewListObjectsV2Paginator(s3client.S3Client, input)

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.TODO())
		if err != nil {
			log.Fatalf("failed to get page, %v", err)
		}

		for _, obj := range page.Contents {
			messageID := *obj.Key

			var count int64
			db.DB.Model(&model.DmarcReportEntry{}).Where("message_id = ?", messageID).Count(&count)
			if count == 0 {
				fmt.Printf("need backfill %s\n", messageID)
				feedback, err := backend.ParseNewMail(messageID)
				fmt.Printf("%+v %+v\n", feedback, err)
				if err != nil {
					continue
				}

				reports := backend.ParseDmarcReport(feedback, messageID)
				fmt.Printf("%+v\n", reports)

				result := db.DB.Create(reports)
				if result.Error != nil {
					fmt.Printf("%+v\n", result.Error)
				}
			} else {
				fmt.Printf("not need backfill %s\n", messageID)
			}

		}
	}
}

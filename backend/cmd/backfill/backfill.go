package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func main() {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}

	svc := s3.NewFromConfig(cfg)

	bucketName := os.Getenv("S3_BUCKET_NAME")
	listObjects(svc, bucketName)
}

func listObjects(svc *s3.Client, bucketName string) {
	input := &s3.ListObjectsV2Input{
		Bucket: &bucketName,
	}

	paginator := s3.NewListObjectsV2Paginator(svc, input)

	fmt.Printf("Listing items in bucket %s:\n", bucketName)

	var keys []string

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.TODO())
		if err != nil {
			log.Fatalf("failed to get page, %v", err)
		}

		for _, obj := range page.Contents {
			keys = append(keys, *obj.Key)
		}
	}

	fmt.Printf("Found %d keys\n", len(keys))
}

package backend

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"golang.org/x/net/html/charset"
)

func ParseNewMail(bucketName, messageId string) (*AggregateReport, error) {
	sdkConfig, err := config.LoadDefaultConfig(context.Background())
	sdkConfig.Region = "us-east-1"
	if err != nil {
		fmt.Println("Couldn't load default configuration. Have you set up your AWS account?")
		fmt.Println(err)
		return nil, err
	}
	s3Client := s3.NewFromConfig(sdkConfig)

	params := &s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(messageId),
	}
	resp, err := s3Client.GetObject(context.Background(), params)

	if err != nil {
		fmt.Printf("Couldn't get object %v:%v. Here's why: %v\n", bucketName, messageId, err)
		return nil, err
	}
	defer resp.Body.Close()

	attachment, err := DmarcReportPrepareAttachment(resp.Body)
	if err != nil {
		fmt.Printf("Couldn't prepare attachment %v:%v. Here's why: %v\n", bucketName, messageId, err)
		return nil, err
	}

	feedback := &AggregateReport{}
	decoder := xml.NewDecoder(attachment)
	decoder.CharsetReader = charset.NewReaderLabel
	if err := decoder.Decode(feedback); err != nil {
		return nil, err
	}

	feedback.MessageId = messageId

	return feedback, err
}

// ExtractZipFile ExtractFile extracts first file from zip archive
func ExtractZipFile(r io.Reader) (io.ReadCloser, error) {
	buf := new(bytes.Buffer)
	_, err := io.Copy(buf, r)
	if err != nil {
		return nil, err
	}

	br := bytes.NewReader(buf.Bytes())
	zip, err := zip.NewReader(br, br.Size())
	if err != nil {
		return nil, err
	}

	if len(zip.File) == 0 {
		return nil, fmt.Errorf("zip: archive is empty: %s", "unreachable")
	}

	return zip.File[0].Open()
}

// DmarcReportPrepareAttachment unzip the dmarc report data, and return the decompressed xml
func DmarcReportPrepareAttachment(f io.Reader) (io.Reader, error) {

	m, err := mail.ReadMessage(f)
	if err != nil {
		return nil, err
	}

	header := m.Header

	mediaType, params, err := mime.ParseMediaType(header.Get("Content-Type"))
	if err != nil {
		return nil, fmt.Errorf("PrepareAttachment: error parsing media type")
	}

	if strings.HasPrefix(mediaType, "multipart/") {
		mr := multipart.NewReader(m.Body, params["boundary"])

		for {
			p, err := mr.NextPart()
			if err == io.EOF {
				return nil, fmt.Errorf("PrepareAttachment: EOF before valid attachment")
			}
			if err != nil {
				return nil, err
			}

			// need to add checks to ensure base64
			partType, _, err := mime.ParseMediaType(p.Header.Get("Content-Type"))
			if err != nil {
				return nil, fmt.Errorf("PrepareAttachment: error parsing media type of part")
			}

			// if gzip
			if strings.HasPrefix(partType, "application/gzip") ||
				strings.HasPrefix(partType, "application/x-gzip") ||
				strings.HasPrefix(partType, "application/gzip-compressed") ||
				strings.HasPrefix(partType, "application/gzipped") ||
				strings.HasPrefix(partType, "application/x-gunzip") ||
				strings.HasPrefix(partType, "application/x-gzip-compressed") ||
				strings.HasPrefix(partType, "gzip/document") {

				decodedBase64 := base64.NewDecoder(base64.StdEncoding, p)
				decompressed, err := gzip.NewReader(decodedBase64)
				if err != nil {
					return nil, err
				}

				return decompressed, nil
			}

			// if zip
			if strings.HasPrefix(partType, "application/zip") || // google style
				strings.HasPrefix(partType, "application/x-zip-compressed") { // yahoo style

				decodedBase64 := base64.NewDecoder(base64.StdEncoding, p)
				decompressed, err := ExtractZipFile(decodedBase64)
				if err != nil {
					return nil, err
				}

				return decompressed, nil
			}

			// if xml
			if strings.HasPrefix(partType, "text/xml") {
				return p, nil
			}

			// if application/octetstream, check filename
			if strings.HasPrefix(partType, "application/octet-stream") {

				if strings.HasSuffix(p.FileName(), ".zip") {
					decodedBase64 := base64.NewDecoder(base64.StdEncoding, p)
					decompressed, err := ExtractZipFile(decodedBase64)
					if err != nil {
						return nil, err
					}

					return decompressed, nil
				}

				if strings.HasSuffix(p.FileName(), ".gz") {
					decodedBase64 := base64.NewDecoder(base64.StdEncoding, p)
					decompressed, _ := gzip.NewReader(decodedBase64)

					return decompressed, nil
				}
			}
		}

	}

	// if gzip
	if strings.HasPrefix(mediaType, "application/gzip") || // proper :)
		strings.HasPrefix(mediaType, "application/x-gzip") || // gmail attachment
		strings.HasPrefix(mediaType, "application/gzip-compressed") ||
		strings.HasPrefix(mediaType, "application/gzipped") ||
		strings.HasPrefix(mediaType, "application/x-gunzip") ||
		strings.HasPrefix(mediaType, "application/x-gzip-compressed") ||
		strings.HasPrefix(mediaType, "gzip/document") {

		decodedBase64 := base64.NewDecoder(base64.StdEncoding, m.Body)
		decompressed, _ := gzip.NewReader(decodedBase64)

		return decompressed, nil

	}

	// if zip
	if strings.HasPrefix(mediaType, "application/zip") || // google style
		strings.HasPrefix(mediaType, "application/x-zip-compressed") { // yahoo style
		decodedBase64 := base64.NewDecoder(base64.StdEncoding, m.Body)
		decompressed, err := ExtractZipFile(decodedBase64)
		if err != nil {
			return nil, err
		}

		return decompressed, nil
	}

	// if xml
	if strings.HasPrefix(mediaType, "text/xml") {
		return m.Body, nil
	}

	return nil, fmt.Errorf("PrepareAttachment: reached the end, no attachment found.")
}

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
	"net"
	"net/mail"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/dmarc-analyzer/dmarc-analyzer/backend/model"
	"github.com/dmarc-analyzer/dmarc-analyzer/backend/s3client"
	"github.com/dmarc-analyzer/dmarc-analyzer/backend/senderbase"
	"golang.org/x/net/html/charset"
)

// ParseNewMail retrieves a DMARC report email from S3 and processes it into a structured format.
// This function performs the following steps:
// 1. Retrieves the email message from the S3 bucket using the provided messageID
// 2. Prepares the attachment by extracting and decompressing it (handles various formats like gzip, zip)
// 3. Decodes the XML content into an AggregateReport structure
//
// Parameters:
//   - messageID: The unique identifier for the email in the S3 bucket
//
// Returns:
//   - *model.AggregateReport: The structured DMARC report data
//   - error: Any error encountered during processing
func ParseNewMail(messageID string) (*model.AggregateReport, error) {

	params := &s3.GetObjectInput{
		Bucket: aws.String(s3client.BucketName),
		Key:    aws.String(messageID),
	}
	resp, err := s3client.S3Client.GetObject(context.Background(), params)

	if err != nil {
		fmt.Printf("Couldn't get object %v:%v. Here's why: %v\n", s3client.BucketName, messageID, err)
		return nil, err
	}
	defer resp.Body.Close()

	attachment, err := DmarcReportPrepareAttachment(resp.Body)
	if err != nil {
		fmt.Printf("Couldn't prepare attachment %v:%v. Here's why: %v\n", s3client.BucketName, messageID, err)
		return nil, err
	}

	feedback, err := DecoderAggregateReport(attachment)

	if err != nil {
		fmt.Printf("Couldn't decode attachment %v:%v. Here's why: %v\n", s3client.BucketName, messageID, err)
		return nil, err
	}

	return feedback, err
}

// DecoderAggregateReport parses the XML content of a DMARC report into a structured format.
// It handles different character encodings that might be present in the XML data.
//
// Parameters:
//   - attachment: An io.Reader containing the XML content of the DMARC report
//
// Returns:
//   - *model.AggregateReport: The structured DMARC report data
//   - error: Any error encountered during XML parsing
func DecoderAggregateReport(attachment io.Reader) (*model.AggregateReport, error) {
	feedback := &model.AggregateReport{}
	decoder := xml.NewDecoder(attachment)
	// Set a charset reader to handle different encodings in the XML
	decoder.CharsetReader = charset.NewReaderLabel
	if err := decoder.Decode(feedback); err != nil {
		return nil, err
	}
	return feedback, nil
}

// ExtractZipFile extracts the first file from a ZIP archive.
// Many DMARC reports are sent as ZIP archives, and this function handles the extraction.
// The function reads the entire ZIP content into memory, creates a reader for it,
// and then extracts the first file in the archive.
//
// Parameters:
//   - r: An io.Reader containing the ZIP archive data
//
// Returns:
//   - io.ReadCloser: A reader for the extracted file content
//   - error: Any error encountered during extraction
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

// DmarcReportPrepareAttachment processes email attachments containing DMARC reports.
// This function handles various compression and encoding formats commonly used by
// different email providers when sending DMARC reports, including:
// - Multipart emails with attachments
// - GZIP compressed files (various MIME types)
// - ZIP compressed files (various MIME types)
// - Plain XML files
// - Application/octet-stream with specific file extensions
//
// The function detects the format, extracts the content, and returns a reader
// with the decompressed XML data ready for parsing.
//
// Parameters:
//   - f: An io.Reader containing the email message
//
// Returns:
//   - io.Reader: A reader containing the decompressed XML content
//   - error: Any error encountered during processing
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

// ParseDmarcReport transforms the XML DMARC report data into database-ready structures.
// It processes each record in the report, enriches it with additional information:
// - Geolocation data from SenderBase
// - Reverse DNS lookups for source IPs
// - Domain information extraction
// - Email Service Provider (ESP) identification
//
// For each record in the DMARC report, this function:
// 1. Retrieves geolocation data for the source IP using SenderBase
// 2. Performs reverse DNS lookups to get hostnames associated with the IP
// 3. Extracts organizational domain information from hostnames
// 4. Identifies Email Service Providers based on domain information
// 5. Combines all this information with the original DMARC report data
// 6. Creates a database-ready structure for each record
//
// Parameters:
//   - feedback: The parsed AggregateReport structure containing the DMARC report data
//   - messageID: The unique identifier for the email message (used as a reference in the database)
//
// Returns:
//   - []*model.DmarcReportEntry: An array of database-ready structures containing the processed report data
func ParseDmarcReport(feedback *model.AggregateReport, messageID string) []*model.DmarcReportEntry {
	reports := make([]*model.DmarcReportEntry, 0, len(feedback.Records))

	// Process each record in the DMARC report
	for i, record := range feedback.Records {
		// Get geolocation data from SenderBase
		sbGeo := senderbase.SenderbaseIPData(record.SourceIP)
		if sbGeo == nil {
			sbGeo = &senderbase.SBGeo{}
		}

		// Perform reverse DNS lookups
		reverseLookupList := model.StringArray(ResolveAddrNames(record.SourceIP))

		// Create the database entry
		reporting := &model.DmarcReportEntry{
			MessageID:       messageID,
			RecordNumber:    int64(i),
			ReportOrgName:   feedback.ReportOrgName,
			Domain:          feedback.Domain,
			Policy:          feedback.Policy,
			SubdomainPolicy: feedback.SubdomainPolicy,
			AlignDKIM:       feedback.AlignDKIM,
			AlignSPF:        feedback.AlignSPF,
			Pct:             feedback.Percentage,
			StartDate:       feedback.DateRangeBegin,
			EndDate:         feedback.DateRangeEnd,
			SourceIP:        model.Inet(net.ParseIP(record.SourceIP)),
			ReverseLookup:   reverseLookupList,
			MessageCount:    record.Count,
			Disposition:     record.Disposition,
			EvalDKIM:        record.EvalDKIM,
			EvalSPF:         record.EvalSPF,
			HeaderFrom:      record.HeaderFrom,
			EnvelopeFrom:    record.EnvelopeFrom,
			EnvelopeTo:      record.EnvelopeTo,
			OrgName:         sbGeo.OrgName,
			OrgID:           sbGeo.OrgID,
			ESP:             sbGeo.ESP,
			SourceHost:      sbGeo.Hostname,
			SourceDomain:    sbGeo.DomainName,
			City:            sbGeo.City,
			State:           sbGeo.State,
			Country:         sbGeo.Country,
			Longitude:       sbGeo.Longitude,
			Latitude:        sbGeo.Latitude,
		}
		for _, dkim := range record.AuthDKIM {
			reporting.AuthDKIMDomain = append(reporting.AuthDKIMDomain, dkim.Domain)
			reporting.AuthDKIMSelector = append(reporting.AuthDKIMSelector, dkim.Selector)
			reporting.AuthDKIMResult = append(reporting.AuthDKIMResult, dkim.Result)
		}
		for _, spf := range record.AuthSPF {
			reporting.AuthSPFDomain = append(reporting.AuthSPFDomain, spf.Domain)
			reporting.AuthSPFScope = append(reporting.AuthSPFScope, spf.Scope)
			reporting.AuthSPFResult = append(reporting.AuthSPFResult, spf.Result)
		}
		for _, po := range record.POReason {
			reporting.POReason = append(reporting.POReason, po.Reason)
			reporting.POComment = append(reporting.POComment, po.Comment)
		}
		reports = append(reports, reporting)
	}
	return reports
}

// AddrNames represents an IP address and its associated DNS names.
// This structure is used for storing the results of reverse DNS lookups.
type AddrNames struct {
	Addr  string   // The IP address
	Names []string // Array of DNS names associated with the IP address
}

// ResolveAddrNames performs a reverse DNS lookup for an IP address.
// It retrieves all DNS names (PTR records) associated with the given IP address.
//
// Parameters:
//   - addr: The IP address to perform the reverse lookup on
//
// Returns:
//   - []string: An array of DNS names associated with the IP address
func ResolveAddrNames(addr string) []string {
	names, _ := net.LookupAddr(addr)
	return names
}

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
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/dmarc-analyzer/dmarc-analyzer/backend/model"
	"github.com/dmarc-analyzer/dmarc-analyzer/backend/senderbase"
	"golang.org/x/net/html/charset"
)

func ParseNewMail(bucketName, messageID string) (*model.AggregateReport, error) {
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
		Key:    aws.String(messageID),
	}
	resp, err := s3Client.GetObject(context.Background(), params)

	if err != nil {
		fmt.Printf("Couldn't get object %v:%v. Here's why: %v\n", bucketName, messageID, err)
		return nil, err
	}
	defer resp.Body.Close()

	attachment, err := DmarcReportPrepareAttachment(resp.Body)
	if err != nil {
		fmt.Printf("Couldn't prepare attachment %v:%v. Here's why: %v\n", bucketName, messageID, err)
		return nil, err
	}

	feedback, err := DecoderAggregateReport(attachment)

	if err != nil {
		fmt.Printf("Couldn't decode attachment %v:%v. Here's why: %v\n", bucketName, messageID, err)
		return nil, err
	}

	return feedback, err
}

func DecoderAggregateReport(attachment io.Reader) (*model.AggregateReport, error) {
	feedback := &model.AggregateReport{}
	decoder := xml.NewDecoder(attachment)
	decoder.CharsetReader = charset.NewReaderLabel
	if err := decoder.Decode(feedback); err != nil {
		return nil, err
	}
	return feedback, nil
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

func ParseDmarcReport(feedback *model.AggregateReport, messageID string) []*model.DmarcReportEntry {
	reports := make([]*model.DmarcReportEntry, 0)
	for i, record := range feedback.Records {
		sbGeo := senderbase.SenderbaseIPData(record.SourceIP)
		reporting := &model.DmarcReportEntry{
			MessageID:         messageID,
			RecordNumber:      int64(i),
			ReportOrgName:     feedback.ReportOrgName,
			Domain:            feedback.Domain,
			Policy:            feedback.Policy,
			SubdomainPolicy:   feedback.SubdomainPolicy,
			AlignDKIM:         feedback.AlignDKIM,
			AlignSPF:          feedback.AlignSPF,
			Pct:               feedback.Percentage,
			StartDate:         feedback.DateRangeBegin,
			EndDate:           feedback.DateRangeEnd,
			SourceIP:          model.Inet(net.ParseIP(record.SourceIP)),
			ReverseLookup:     ResolveAddrNames(record.SourceIP),
			MessageCount:      record.Count,
			Disposition:       record.Disposition,
			EvalDKIM:          record.EvalDKIM,
			EvalSPF:           record.EvalSPF,
			HeaderFrom:        record.HeaderFrom,
			EnvelopeFrom:      record.EnvelopeFrom,
			EnvelopeTo:        record.EnvelopeTo,
			OrgName:           sbGeo.OrgName,
			OrgID:             sbGeo.OrgID,
			ESP:               sbGeo.ESP,
			HostName:          sbGeo.Hostname,
			DomainName:        sbGeo.DomainName,
			HostNameMatchesIP: sbGeo.HostnameMatchesIP,
			City:              sbGeo.City,
			State:             sbGeo.State,
			Country:           sbGeo.Country,
			Longitude:         sbGeo.Longitude,
			Latitude:          sbGeo.Latitude,
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

// AddrNames includes address and its names array
type AddrNames struct {
	Addr  string
	Names []string
}

// ResolveAddrNames returns a struct containging an address and a list of names mapping to it
func ResolveAddrNames(addr string) []string {
	names, _ := net.LookupAddr(addr)
	return names
}

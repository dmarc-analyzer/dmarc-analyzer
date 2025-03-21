// Package util provides utility functions for the DMARC analyzer.
// It includes functions for domain processing, date handling, and other common operations.
package util

import (
	"fmt"
	"strings"

	"golang.org/x/net/publicsuffix"
)

// GetOrgDomain returns the organizational domain for the input domain.
// The organizational domain is the registrable domain plus one label to the left.
// For example, the organizational domain of "mail.example.com" is "example.com".
//
// This function uses the publicsuffix package to determine the registrable domain
// according to the Public Suffix List (PSL), which contains information about
// top-level domains and their registration policies.
//
// Parameters:
//   - domain: The domain name to process
//
// Returns:
//   - orgDomain: The organizational domain
//   - err: Any error encountered during processing
func GetOrgDomain(domain string) (orgDomain string, err error) {

	// trailing periods should be stripped before splitting:
	domain = strings.TrimRight(domain, ".")

	// resolve organizational domain:
	icannDomain, isIcann := publicsuffix.PublicSuffix(domain)

	if !isIcann {
		// bad domain spec
		//  -- not sure how we will ever be here if HostDomain is compliant
		err = fmt.Errorf("bad organizational domain: %s", domain)
		return
	}

	domainLabels := strings.Split(domain, ".")
	icannLabels := strings.Split(icannDomain, ".")
	icl := len(icannLabels)
	dcl := len(domainLabels)
	orgDomain = ""

	if dcl-icl <= 1 {
		// domain is org domain:
		orgDomain = domain
		return
	}
	for j := dcl - icl - 1; j < dcl; j++ {
		orgDomain = orgDomain + domainLabels[j] + "."
	}
	orgDomain = strings.TrimRight(orgDomain, ".")
	return
}

// GetESP returns email service provider
// https://sendview.io/esp
// https://sendview.io/top-email-marketing-services-2020
func GetESP(orgDomain string) string {
	var esp string
	switch orgDomain {
	case "amazonses.com":
		esp = "Amazon SES"
	case "google.com":
		esp = "Google Mail"
	case "rsgsv.net", "mcsv.net", "mcdlv.net":
		esp = "MailChimp"
	}
	return esp
}

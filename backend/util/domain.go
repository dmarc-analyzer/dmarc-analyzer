package util

import (
	"fmt"
	"strings"

	"golang.org/x/net/publicsuffix"
)

// GetOrgDomain returns the org domain for the input domain according to the
// mechanisms of code in the publicsuffix package
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

	return
}

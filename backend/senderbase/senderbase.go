// Package senderbase provides functionality to query the SenderBase database
// for information about IP addresses, including organization details and geolocation data.
// SenderBase is a reputation database that provides information about email senders.
package senderbase

import (
	"fmt"
	"log"
	"net"
	"sort"
	"strings"
)

// SBGeo represents geolocation and organizational data for an IP address.
// This structure contains information retrieved from SenderBase about the sender.
type SBGeo struct {
	OrgName     string // Organization name associated with the IP
	OrgID       string // Organization ID in SenderBase
	OrgCategory string // Category of the organization (e.g., ISP, hosting provider)
	ESP         string // Email Service Provider name if applicable
	Hostname    string // Hostname associated with the IP
	DomainName  string // Domain name associated with the IP
	City        string // City location of the IP
	State       string // State/province location of the IP
	Country     string // Country location of the IP
	Longitude   string // Longitude coordinate of the IP
	Latitude    string // Latitude coordinate of the IP
}

// revIP4 represents an IPv4 address in reverse byte order.
// This is used for SenderBase DNS lookups which require reversed IP format.
type revIP4 struct {
	Byte   [4]byte // The IP address bytes in reverse order
	String string  // String representation of the reversed IP
}

// byteReverseIP4 translates an IPv4 address into reverse byte order.
// This is necessary for querying the SenderBase DNS system which uses
// reversed IP addresses in its query format.
//
// Parameters:
//   - ip: The IPv4 address to reverse
//
// Returns:
//   - revip: A revIP4 structure containing the reversed IP in both byte and string formats
func byteReverseIP4(ip net.IP) (revip revIP4) {
	for j := 0; j < len(ip); j++ {
		revip.Byte[len(ip)-j-1] = ip[j]
		revip.String = fmt.Sprintf("%d.%s", ip[j], revip.String)
	}

	revip.String = strings.TrimRight(revip.String, ".")

	return
}

// SenderbaseIPData queries the SenderBase database to retrieve information about an IP address.
// It performs a DNS lookup against the SenderBase DNS system to get organization and geolocation data.
//
// Parameters:
//   - sip: The IP address to query as a string
//
// Returns:
//   - *SBGeo: A pointer to an SBGeo structure containing the retrieved information,
//     or nil if the lookup failed or the IP is not an IPv4 address
func SenderbaseIPData(sip string) *SBGeo {

	// convert from string input to net.IP:
	ip := net.ParseIP(sip).To4()
	if ip == nil {

		ip = net.ParseIP(sip).To16()
		if ip == nil {
			log.Printf("invalid address: %s\n", sip)
			return nil
		}
		return GetIPV6Data(sip)
	}

	// reverse the byte-order of IP:
	srevip := byteReverseIP4(ip)

	// senderbase ip-specific domain to query:
	domain := fmt.Sprintf("%s.query.senderbase.org", srevip.String)

	// perform the lookup:
	txtRecords, errLookupTXT := net.LookupTXT(domain)
	var err error
	if errLookupTXT != nil {
		err = fmt.Errorf("SBIPD - errLookupTXT:  %s\n%s", domain, errLookupTXT)
		log.Println(err)
		return nil
	}
	if len(txtRecords) < 1 {
		err = fmt.Errorf("no TXT records found for IP %s\n%s", domain, sip)
		log.Println(err)
		return nil
	}

	rr := txtRecords[0]

	// handle multiple TXT records:
	sbStr := rr
	if len(txtRecords) > 1 {
		sort.Slice(txtRecords, func(i, j int) bool { return txtRecords[i][0] < txtRecords[j][0] })
		sbStr = ""
		for j := range txtRecords {
			// each TXT leads with '[0-9]-'...strip away this 2-char prefix
			txtRecords[j] = txtRecords[j][2:len(txtRecords[j])] // this could use regex improvement
			sbStr = fmt.Sprintf("%s%s", sbStr, txtRecords[j])
		}
	}
	sbFields := strings.Split(sbStr, "|")
	sbMap := map[string]string{}
	for j := range sbFields {
		sbm := strings.Split(sbFields[j], "=")
		sbMap[sbm[0]] = sbm[1]
	}

	sbGeo := &SBGeo{}
	sbGeo.OrgName = sbMap["1"]
	sbGeo.OrgID = sbMap["4"]
	sbGeo.OrgCategory = sbMap["5"]
	sbGeo.Hostname = strings.ToLower(sbMap["20"])
	sbGeo.DomainName = strings.ToLower(sbMap["21"])
	sbGeo.City = sbMap["50"]
	sbGeo.State = sbMap["51"]
	sbGeo.Country = sbMap["53"]
	sbGeo.Longitude = sbMap["54"]
	sbGeo.Latitude = sbMap["55"]

	// If the ESP field is not set but we have organization information,
	// try to identify common Email Service Providers based on organization name
	if sbGeo.ESP == "" && sbGeo.OrgName != "" {
		orgName := strings.ToLower(sbGeo.OrgName)
		if strings.Contains(orgName, "google") || strings.Contains(orgName, "gmail") {
			sbGeo.ESP = "Google Mail"
		} else if strings.Contains(orgName, "amazon") || strings.Contains(orgName, "aws") {
			sbGeo.ESP = "Amazon SES"
		} else if strings.Contains(orgName, "mailchimp") {
			sbGeo.ESP = "MailChimp"
		}
	}

	return sbGeo
}

func GetIPV6Data(sip string) *SBGeo {
	sbGeo := &SBGeo{}
	hostnames, err := net.LookupAddr(sip)
	if err != nil {
		log.Printf("GetIPV6Data LookupAddr err: %s", err)
		return sbGeo
	}
	if len(hostnames) > 0 {
		hostname := hostnames[0]
		hostname = strings.TrimSuffix(hostname, ".")
		sbGeo.Hostname = hostname
		if strings.HasSuffix(hostname, "outlook.com") {
			sbGeo.DomainName = "outlook.com"
			sbGeo.ESP = "Outlook"
		}
	}
	return sbGeo
}

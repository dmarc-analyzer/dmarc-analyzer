package senderbase

import (
	"fmt"
	"log"
	"net"
	"sort"
	"strings"
)

type SBGeo struct {
	OrgName           string `json:"org_name"`
	OrgID             string `json:"org_id"`
	OrgCategory       string `json:"org_category"`
	Hostname          string `json:"hostname"`
	DomainName        string `json:"domain_name"`
	HostnameMatchesIP string `json:"hostname_matches_ip"`
	City              string `json:"city"`
	State             string `json:"state"`
	Country           string `json:"country"`
	Longitude         string `json:"longitude"`
	Latitude          string `json:"latitude"`
}

type revIP4 struct {
	Byte   [4]byte
	String string
}

// ByteReverseIP4 translates an IP4 into reverse byte order
func byteReverseIP4(ip net.IP) (revip revIP4) {

	for j := 0; j < len(ip); j++ {
		revip.Byte[len(ip)-j-1] = ip[j]
		revip.String = fmt.Sprintf("%d.%s", ip[j], revip.String)
	}

	revip.String = strings.TrimRight(revip.String, ".")

	return
}

// SenderbaseIPData query the senderbase to find out the org name of ip
func SenderbaseIPData(sip string) *SBGeo {

	// convert from string input to net.IP:
	ip := net.ParseIP(sip).To4()
	if ip == nil {
		log.Println("ip6 address")
		return nil
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
	sbGeo.HostnameMatchesIP = sbMap["22"]
	sbGeo.City = sbMap["50"]
	sbGeo.State = sbMap["51"]
	sbGeo.Country = sbMap["53"]
	sbGeo.Longitude = sbMap["54"]
	sbGeo.Latitude = sbMap["55"]

	return sbGeo
}

package util

import (
	"fmt"
	"testing"
)

func TestGetOrgDomain(t *testing.T) {
	gotOrgDomain, err := GetOrgDomain("mail-oo1-f48.google.com.")
	fmt.Printf("gotOrgDomain: %s error: %+v", gotOrgDomain, err)
}

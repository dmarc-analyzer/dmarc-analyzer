package senderbase

import (
	"fmt"
	"testing"
)

func TestSenderbaseIPData(t *testing.T) {
	sbGeo := SenderbaseIPData("209.85.220.41")
	fmt.Printf("%+v\n", sbGeo)
}

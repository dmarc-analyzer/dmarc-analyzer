package senderbase

import (
	"fmt"
	"testing"
)

func TestSenderbaseIPData(t *testing.T) {
	sbGeo := SenderbaseIPData("209.85.220.41")
	fmt.Printf("%+v\n", sbGeo)
}

func TestSenderbaseIPV6Data(t *testing.T) {
	sbGeo := SenderbaseIPData("2a01:111:f403:38::204")
	fmt.Printf("%+v\n", sbGeo)
}

func TestSenderbaseIPDataInvalid(t *testing.T) {
	sbGeo := SenderbaseIPData("1.1")
	fmt.Printf("%+v\n", sbGeo)
}

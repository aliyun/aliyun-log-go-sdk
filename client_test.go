package sls

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSignv4Acdr does not talk to the network: it only inspects the
// AuthVersion / Region the constructor derives from the endpoint.
func TestSignv4Acdr(t *testing.T) {
	{
		client := CreateNormalInterface("https://xx-test-acdr-ut-1-intranet.log.aliyuncs.com", "", "", "")
		c := client.(*Client)
		assert.Equal(t, c.Region, "xx-test-acdr-ut-1")
		assert.Equal(t, c.AuthVersion, AuthV4)
	}

	{
		client := CreateNormalInterface("https://cn-hangzhou-intranet.log.aliyuncs.com", "", "", "")
		c := client.(*Client)
		assert.Equal(t, c.Region, "")
		assert.EqualValues(t, c.AuthVersion, "")
	}
}

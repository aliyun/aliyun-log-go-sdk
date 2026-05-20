package sls

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestShouldRefresh(t *testing.T) {
	callCnt := 0
	now := time.Now()
	id, secret, token := "a1", "b1", "c1"
	expiration := now.Add(time.Hour)
	var mockErr error
	updateFunc := func() (string, string, string, time.Time, error) {
		callCnt++
		return id, secret, token, expiration, mockErr
	}
	adp := NewUpdateFuncProviderAdapter(updateFunc)
	assert.True(t, adp.shouldRefresh())
	cred := &tempCredentials{
		Credentials: Credentials{
			AccessKeyID:     id,
			AccessKeySecret: secret,
			SecurityToken:   token,
		},
		Expiration: expiration,
	}
	adp.cred.Store(cred)
	assert.False(t, adp.shouldRefresh())

	// expired
	cred.Expiration = now.Add(-time.Hour)
	adp.cred.Store(cred)
	assert.True(t, adp.shouldRefresh())

	// not expire but fetch ahead
	cred.Expiration = now.Add(-adp.fetchAhead).Add(-time.Second)
	adp.cred.Store(cred)
	assert.True(t, adp.shouldRefresh())
}

func TestUpdateFuncAdapter(t *testing.T) {
	callCnt := 0
	now := time.Now()
	id, secret, token := "a1", "b1", "c1"
	expiration := now.Add(time.Hour)
	var mockErr error
	updateFunc := func() (string, string, string, time.Time, error) {
		callCnt++
		return id, secret, token, expiration, mockErr
	}
	adp := NewUpdateFuncProviderAdapter(updateFunc)
	adpRetry := UPDATE_FUNC_RETRY_TIMES
	// first time fetch failed
	callCnt = 0
	mockErr = errors.New("mock err")
	{
		_, err := adp.GetCredentials()
		assert.Equal(t, 1+adpRetry, callCnt)
		assert.Error(t, err)
	}

	// first fetch success
	callCnt = 0
	mockErr = nil
	{
		cred, err := adp.GetCredentials()
		assert.Equal(t, 1, callCnt)
		assert.NoError(t, err)
		assert.Equal(t, cred.AccessKeyID, id)
		assert.Equal(t, cred.AccessKeySecret, secret)
		assert.Equal(t, cred.SecurityToken, token)
	}

	// fetch again, use cached cred
	callCnt = 0
	mockErr = nil
	id = "a2"
	{
		cred, err := adp.GetCredentials()
		assert.NoError(t, err)
		assert.Equal(t, 0, callCnt)
		assert.Equal(t, cred.AccessKeyID, "a1")
	}

	// expired, fetch new
	callCnt = 0
	mockErr = nil
	id = "a2"
	adp.cred.Load().(*tempCredentials).Expiration = now.Add(-time.Hour)
	{
		cred, err := adp.GetCredentials()
		assert.NoError(t, err)
		assert.Equal(t, 1, callCnt)
		assert.Equal(t, cred.AccessKeyID, "a2")
	}

	// fetch failed test, use last cred
	callCnt = 0
	adp.cred.Load().(*tempCredentials).Expiration = now.Add(-time.Hour)
	mockErr = errors.New("mock err")
	{
		cred, err := adp.GetCredentials()
		assert.NoError(t, err)
		assert.Equal(t, 1+adpRetry, callCnt)
		assert.Equal(t, cred.AccessKeyID, "a2")
	}

	callCnt = 0
	adp.cred.Load().(*tempCredentials).Expiration = expiration
	mockErr = nil
	{
		cred, err := adp.GetCredentials()
		assert.NoError(t, err)
		assert.Equal(t, 0, callCnt)
		assert.Equal(t, cred.AccessKeyID, "a2")
	}

	// fetch in advance, fetch a new one
	// use fetchCredentailsAhead
	callCnt = 0
	id = "a3"
	cred := adp.cred.Load().(*tempCredentials)
	adp.fetchAhead = time.Hour * 10
	cred.Expiration = now.Add(time.Hour)
	mockErr = nil
	{
		cred, err := adp.GetCredentials()
		assert.NoError(t, err)
		assert.Equal(t, 1, callCnt)
		assert.Equal(t, cred.AccessKeyID, "a3")
	}
}

package sls

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"
)

func TestGetLogsTestSuite(t *testing.T) {
	suite.Run(t, new(GetLogsTestSuite))
}

// GetLogsTestSuite collects the pure unit tests around the v3 ->
// v2 response conversion plumbing. The e2e variant lives in
// log_store_e2e_test.go behind the `e2e` build tag.
type GetLogsTestSuite struct {
	suite.Suite
}

func (s *GetLogsTestSuite) TestConstructQueryInfo() {
	v3Meta := &GetLogsV3ResponseMeta{
		Keys:            nil,
		Terms:           nil,
		Marker:          nil,
		Mode:            nil,
		PhraseQueryInfo: nil,
		Shard:           nil,
		ScanBytes:       nil,
		IsAccurate:      nil,
		ColumnTypes:     nil,
		Highlights:      nil,
	}
	contents, err := v3Meta.constructQueryInfo()
	s.Require().NoError(err)
	s.Equal("{}", contents)
	b := false
	v3Meta.IsAccurate = &b
	contents, err = v3Meta.constructQueryInfo()
	s.Require().NoError(err)
	s.Equal("{\"isAccurate\":0}", contents)
	b = true
	contents, err = v3Meta.constructQueryInfo()
	s.Require().NoError(err)
	s.Equal("{\"isAccurate\":1}", contents)

	v3Meta.Keys = make([]string, 0)
	shard := 0
	v3Meta.Shard = &shard
	contents, err = v3Meta.constructQueryInfo()
	s.Require().NoError(err)

	s.Equal("{\"shard\":0,\"isAccurate\":1}", contents)
}

func (s *GetLogsTestSuite) TestMarshalLines() {
	logs := []map[string]string{
		{
			"key1": "va1",
			"key2": " sdsadsa",
		},
		{
			"keOIIO y1": "NKJ*((*va1",
			"ke y2":     " sdsadsa",
		},
		{
			"keA DSy1": "va 2>>122e",
			"key2":     " sds2adsa",
		},
	}
	data, err := json.Marshal(logs)
	s.Require().NoError(err)
	var msg []json.RawMessage
	err = json.Unmarshal(data, &msg)
	s.Require().NoError(err)
}

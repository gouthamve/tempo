package wal

import (
	"bytes"
	"io/ioutil"
	"math"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/grafana/tempo/tempodb/encoding"

	"github.com/stretchr/testify/assert"
)

func TestCompactorBlockError(t *testing.T) {
	_, err := newCompactorBlock(uuid.New(), "", 0, 0, nil, "", 0)
	assert.Error(t, err)
}

func TestCompactorBlockWrite(t *testing.T) {
	tempDir, err := ioutil.TempDir("/tmp", "")
	defer os.RemoveAll(tempDir)
	assert.NoError(t, err, "unexpected error creating temp dir")

	walCfg := &Config{
		Filepath:        tempDir,
		IndexDownsample: 3,
		BloomFP:         .01,
	}
	wal, err := New(walCfg)
	assert.NoError(t, err)

	metas := []*encoding.BlockMeta{
		{
			StartTime: time.Unix(10000, 0),
			EndTime:   time.Unix(20000, 0),
		},
		{
			StartTime: time.Unix(15000, 0),
			EndTime:   time.Unix(25000, 0),
		},
	}

	numObjects := (rand.Int() % 20) + 1
	cb, err := wal.NewCompactorBlock(uuid.New(), testTenantID, metas, numObjects)
	assert.NoError(t, err)

	var minID encoding.ID
	var maxID encoding.ID

	ids := make([][]byte, 0)
	for i := 0; i < numObjects; i++ {
		id := make([]byte, 16)
		_, err = rand.Read(id)
		assert.NoError(t, err)

		object := make([]byte, rand.Int()%1024)
		_, err = rand.Read(object)
		assert.NoError(t, err)

		ids = append(ids, id)

		err = cb.Write(id, object)
		assert.NoError(t, err)

		if len(minID) == 0 || bytes.Compare(id, minID) == -1 {
			minID = id
		}
		if len(maxID) == 0 || bytes.Compare(id, maxID) == 1 {
			maxID = id
		}
	}
	cb.Complete()

	assert.Equal(t, numObjects, cb.Length())

	// test meta
	meta := cb.BlockMeta()

	assert.Equal(t, time.Unix(10000, 0), meta.StartTime)
	assert.Equal(t, time.Unix(25000, 0), meta.EndTime)
	assert.Equal(t, minID, meta.MinID)
	assert.Equal(t, maxID, meta.MaxID)
	assert.Equal(t, testTenantID, meta.TenantID)
	assert.Equal(t, numObjects, meta.TotalObjects)

	// bloom
	bloom := cb.BloomFilter()
	for _, id := range ids {
		has := bloom.Test(id)
		assert.True(t, has)
	}

	records := cb.Records()
	assert.Equal(t, math.Ceil(float64(numObjects)/3), float64(len(records)))
}

package cgozstd

import (
	"sync"

	"github.com/DataDog/zstd"
)

type ZstdCompressor struct {
	ctxPool sync.Pool
	level   int
}

func NewZstdCompressor(level int) *ZstdCompressor {
	res := &ZstdCompressor{
		level: level,
	}
	res.ctxPool = sync.Pool{
		New: func() interface{} {
			return zstd.NewCtx()
		},
	}
	return res
}

func (c *ZstdCompressor) Compress(src, dst []byte) ([]byte, error) {
	zstdCtx := c.ctxPool.Get().(zstd.Ctx)
	defer c.ctxPool.Put(zstdCtx)
	return zstdCtx.CompressLevel(dst, src, c.level)
}

func (c *ZstdCompressor) Decompress(src, dst []byte) ([]byte, error) {
	zstdCtx := c.ctxPool.Get().(zstd.Ctx)
	defer c.ctxPool.Put(zstdCtx)
	return zstdCtx.Decompress(dst, src)
}

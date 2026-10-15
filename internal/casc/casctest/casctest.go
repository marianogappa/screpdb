// Package casctest builds a small fake StarCraft: Remastered CASC install for tests.
package casctest

import (
	"bytes"
	"compress/zlib"
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/marianogappa/screpdb/internal/iofacade"
)

const (
	indexBuckets      = 16
	indexVersion      = 7
	eKeyIndexLen      = 9
	storageOffsetLen  = 5
	encodedSizeLen    = 4
	indexEntriesAt    = 0x28
	dataHeaderLen     = 0x1e
	blteChunkInfoLen  = 24
	blteTableFlags    = 0x0f
	encodingKeyLen    = 16
	pageTableEntryLen = 2 * encodingKeyLen
)

type key = [16]byte

func reversed(b []byte) []byte {
	out := make([]byte, len(b))
	for i, c := range b {
		out[len(b)-1-i] = c
	}
	return out
}

const testOffsetBits = 30

type fakeStorage struct {
	t     *testing.T
	dir   string
	data  bytes.Buffer
	index [indexBuckets][]byte
}

func blteRaw(content []byte) []byte {
	return append([]byte("BLTE\x00\x00\x00\x00N"), content...)
}

// BLTEChunked encodes content as a raw chunk and a zlib chunk.
func BLTEChunked(content []byte) []byte {
	half := len(content) / 2
	var z bytes.Buffer
	w := zlib.NewWriter(&z)
	w.Write(content[half:])
	w.Close()
	chunks := [][]byte{append([]byte{'N'}, content[:half]...), append([]byte{'Z'}, z.Bytes()...)}
	sizes := []int{half, len(content) - half}
	header := 12 + len(chunks)*blteChunkInfoLen
	out := []byte("BLTE")
	out = binary.BigEndian.AppendUint32(out, uint32(header))
	out = append(out, blteTableFlags, 0, 0, byte(len(chunks)))
	for i, c := range chunks {
		out = binary.BigEndian.AppendUint32(out, uint32(len(c)))
		out = binary.BigEndian.AppendUint32(out, uint32(sizes[i]))
		sum := md5.Sum(c)
		out = append(out, sum[:]...)
	}
	for _, c := range chunks {
		out = append(out, c...)
	}
	return out
}

// store writes one entry into data.000 and its index bucket, and returns its encoded key.
func (f *fakeStorage) store(encoded []byte) key {
	ekey := md5.Sum(encoded)
	offset := f.data.Len()
	header := make([]byte, dataHeaderLen)
	copy(header[16-eKeyIndexLen:16], reversed(ekey[:eKeyIndexLen]))
	binary.LittleEndian.PutUint32(header[16:], uint32(dataHeaderLen+len(encoded)))
	f.data.Write(header)
	f.data.Write(encoded)

	bucket := 0
	for _, c := range ekey[:eKeyIndexLen] {
		bucket ^= int(c)
	}
	bucket = (bucket&0xf ^ bucket>>4) & 0xf
	entry := append([]byte(nil), ekey[:eKeyIndexLen]...)
	storage := uint64(offset)
	for i := storageOffsetLen - 1; i >= 0; i-- {
		entry = append(entry, byte(storage>>(8*i)))
	}
	entry = binary.LittleEndian.AppendUint32(entry, uint32(dataHeaderLen+len(encoded)))
	f.index[bucket] = append(f.index[bucket], entry...)
	return ekey
}

// EncodingTable builds a one-page encoding table.
func EncodingTable(entries map[key]key) []byte {
	page := make([]byte, 4096)
	p := 0
	for ck, ek := range entries {
		page[p] = 1
		copy(page[p+6:], ck[:])
		copy(page[p+6+encodingKeyLen:], ek[:])
		p += 6 + 2*encodingKeyLen
	}
	h := []byte{'E', 'N', 1, encodingKeyLen, encodingKeyLen, 0, 4, 0, 4}
	h = binary.BigEndian.AppendUint32(h, 1)
	h = binary.BigEndian.AppendUint32(h, 0)
	h = append(h, 0)
	h = binary.BigEndian.AppendUint32(h, 0)
	h = append(h, make([]byte, pageTableEntryLen)...)
	return append(h, page...)
}

func (f *fakeStorage) write(name string, b []byte) {
	path := filepath.Join(f.dir, name)
	if err := iofacade.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := iofacade.WriteFile(path, b, 0o644); err != nil {
		f.t.Fatal(err)
	}
}

// BuildInstall writes an install holding files (root path to content) and returns its folder.
func BuildInstall(t *testing.T, files map[string][]byte) string {
	t.Helper()
	f := &fakeStorage{t: t, dir: t.TempDir()}
	encoding := map[key]key{}
	var root strings.Builder
	i := 0
	for name, content := range files {
		ck := md5.Sum(content)
		encoded := blteRaw(content)
		if i%2 == 1 {
			encoded = BLTEChunked(content)
		}
		encoding[ck] = f.store(encoded)
		fmt.Fprintf(&root, "%s|%x\n", name, ck)
		i++
	}
	rootContent := []byte(root.String())
	rootKey := md5.Sum(rootContent)
	encoding[rootKey] = f.store(blteRaw(rootContent))
	table := EncodingTable(encoding)
	tableKey := md5.Sum(table)
	tableEKey := f.store(blteRaw(table))

	config := fmt.Sprintf("# Build Configuration\n\nroot = %x\nencoding = %x %x\nbuild-name = 9.9.9-test\nbuild-product = StarCraft1\n", rootKey, tableKey, tableEKey)
	buildKey := hex.EncodeToString(md5.New().Sum([]byte("build")))[:32]
	f.write(filepath.Join("Data", "config", buildKey[0:2], buildKey[2:4], buildKey), []byte(config))
	f.write(".build.info", []byte("Branch!STRING:0|Active!DEC:1|Build Key!HEX:16\r\nus|0|ffffffffffffffffffffffffffffffff\r\nus|1|"+buildKey+"\r\n"))
	f.write(filepath.Join("Data", "data", "data.000"), f.data.Bytes())
	for bucket, entries := range f.index {
		b := make([]byte, indexEntriesAt)
		binary.LittleEndian.PutUint16(b[8:], indexVersion)
		b[10], b[12], b[13], b[14], b[15] = byte(bucket), encodedSizeLen, storageOffsetLen, eKeyIndexLen, testOffsetBits
		binary.LittleEndian.PutUint32(b[0x20:], uint32(len(entries)))
		f.write(filepath.Join("Data", "data", fmt.Sprintf("%02x00000001.idx", bucket)), append(b, entries...))
		f.write(filepath.Join("Data", "data", fmt.Sprintf("%02x00000000.idx", bucket)), []byte("stale"))
	}
	return f.dir
}

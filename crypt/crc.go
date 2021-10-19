package crypt

import (
	"hash/crc64"
	"math/rand"
	"strconv"
)

type (

	// CRCGenerator --
	// (PSEUDO-RANDOM REPLICABLE SEEDED GENERATION)
	CRCGenerator struct {
		*rand.Rand
		poly          uint64
		masterkey     string
		masterinthash uint64
		masterseed    int64
		table         *crc64.Table
	}
)

func NewCRCGenerator(generatorPoly uint64) *CRCGenerator {
	return &CRCGenerator{
		poly:  generatorPoly,
		table: crc64.MakeTable(generatorPoly),
	}
}

func (g *CRCGenerator) Init(seed string) {
	g.masterinthash = g.generateHashInt(seed)
	g.masterkey = g.GenChecksum(seed)
	g.masterseed = int64(g.masterinthash)
	g.Rand = rand.New(rand.NewSource(g.masterseed))
	g.Seed(g.masterseed)
}

func (g *CRCGenerator) GenChecksum(seed string) string {
	hashInt := g.generateHashInt(seed)
	return strconv.FormatUint(hashInt, 16)
}

func (g *CRCGenerator) generateHashInt(seed string) uint64 {
	return crc64.Checksum([]byte(seed), g.table)
}

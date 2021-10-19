package crypt

//

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/google/uuid"
)

type (
	Crypt struct {
		key []byte
		crc *CRCGenerator
	}
)

var ()

// New --
// CRYPTO KEY 32 LEN BYTE KEY
// generatorPoly - UINT64 POLY FOR CRC TABLE (PSEUDO-RANDOM REPLICATABLE SEEDED GENERATION)
// [DEPRECATED] - InitGenerator NEEDS TO BE CALLED WITH YOUR PROVIDED SEED TO SEED THE RANDOM GENERATOR FOR REPLICATABLE GENERATION
func New(key []byte, generatorPoly uint64) *Crypt {
	// tmpKey := make([]byte, 32, '0')
	tmpKey := make([]byte, 32)
	for i := range tmpKey {
		if i < len(key) {
			tmpKey[i] = key[i]
		} else {
			tmpKey[i] = '0'
		}
	}
	key = tmpKey
	// log.Println("KEY:", string(key))
	crcGen := NewCRCGenerator(generatorPoly)
	crcGen.Init(string(key))
	return &Crypt{
		key: key,
		crc: crcGen,
	}

}

func (c *Crypt) InitGenerator(seed string) {
	c.crc.Init(seed)
}

func (c *Crypt) Checksum(seed string) string {
	return c.crc.GenChecksum(seed)
}

func (c *Crypt) GetCRCGenerator() *CRCGenerator {
	return c.crc
}

func (c *Crypt) newHash(b []byte) string {
	hasher := md5.New()
	hasher.Write(b)
	return hex.EncodeToString(hasher.Sum(nil))
}

func (c *Crypt) Encrypt(data []byte) ([]byte, error) {
	block, err := aes.NewCipher([]byte(c.newHash(c.key)))
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	ciphertext := gcm.Seal(nonce, nonce, data, nil)
	return ciphertext, nil
}

func (c *Crypt) Decrypt(data []byte) ([]byte, error) {
	key := []byte(c.newHash(c.key))
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if nonceSize > len(data) {
		return nil, errors.New(fmt.Sprintf("nonce size > len data %d > %d", nonceSize, len(data)))
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}
	return plaintext, nil
}

func Encode(str string) string {
	return base64.URLEncoding.EncodeToString([]byte(str))
}

func Decode(str string) string {
	b, err := base64.URLEncoding.DecodeString(str)
	if err != nil {
		b = []byte{}
	}
	return string(b)
}

// nanoserver pw | salt generation

func (c *Crypt) initHash(str, salt string) string {
	buf := &bytes.Buffer{}
	fmt.Fprintf(buf, "%s%x%s", salt, str, salt)
	result1 := sha1.Sum(buf.Bytes())

	buf.Reset()

	fmt.Fprintf(buf, "%s%s%x%s%s", str, salt, result1, salt, str)
	result2 := sha1.Sum(buf.Bytes())

	buf.Reset()
	fmt.Fprintf(buf, "%x", result2)
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func (c *Crypt) NewHashAndSalt(str string) (hash, salt string) {
	salt = strings.Replace(uuid.New().String(), "-", "", -1)
	hash = c.initHash(str, salt)
	return hash, salt
}

func (c *Crypt) VerifyPassword(pwd, salt, hash string) bool {
	return c.initHash(pwd, salt) == hash
}

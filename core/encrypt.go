package core

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	cryptorand "crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var PwdKey = []byte("xoxoslslgrgriiiixoxoslslgrgriiii")

func pkcs7Padding(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	padText := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(data, padText...)
}

func pkcs7UnPadding(data []byte) ([]byte, error) {
	length := len(data)
	if length == 0 {
		return nil, errors.New("加密字符串错误！")
	}
	unPadding := int(data[length-1])
	if unPadding == 0 || unPadding > length {
		return nil, errors.New("加密字符串填充错误！")
	}
	for _, value := range data[length-unPadding:] {
		if int(value) != unPadding {
			return nil, errors.New("加密字符串填充错误！")
		}
	}
	return data[:(length - unPadding)], nil
}

func AesEncrypt(data []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	blockSize := block.BlockSize()
	encryptBytes := pkcs7Padding(data, blockSize)
	crypted := make([]byte, len(encryptBytes))
	blockMode := cipher.NewCBCEncrypter(block, key[:blockSize])
	blockMode.CryptBlocks(crypted, encryptBytes)
	return crypted, nil
}

func AesDecrypt(data []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	blockSize := block.BlockSize()
	if len(data) == 0 || len(data)%blockSize != 0 {
		return nil, errors.New("加密字符串长度错误！")
	}
	blockMode := cipher.NewCBCDecrypter(block, key[:blockSize])
	crypted := make([]byte, len(data))
	blockMode.CryptBlocks(crypted, data)
	crypted, err = pkcs7UnPadding(crypted)
	if err != nil {
		return nil, err
	}
	return crypted, nil
}

func EncryptByAes(data []byte) (string, error) {
	block, err := aes.NewCipher(PwdKey)
	if err != nil {
		return "", err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := cryptorand.Read(nonce); err != nil {
		return "", err
	}
	sealed := aead.Seal(nil, nonce, data, nil)
	payload := append(nonce, sealed...)
	return "v2:" + base64.StdEncoding.EncodeToString(payload), nil
}

func DecryptByAes(data string) ([]byte, error) {
	if strings.HasPrefix(data, "v2:") {
		payload, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(data, "v2:"))
		if err != nil {
			return nil, err
		}
		block, err := aes.NewCipher(PwdKey)
		if err != nil {
			return nil, err
		}
		aead, err := cipher.NewGCM(block)
		if err != nil {
			return nil, err
		}
		if len(payload) < aead.NonceSize() {
			return nil, errors.New("加密字符串长度错误！")
		}
		return aead.Open(nil, payload[:aead.NonceSize()], payload[aead.NonceSize():], nil)
	}
	dataByte, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return nil, err
	}
	return AesDecrypt(dataByte, PwdKey)
}

func halfEct(str string) string {
	ss := regexp.MustCompile(`/\*hidden\*/([\s\S]+?)/\*neddih\*/`).FindAllString(str, -1)
	for _, v := range ss {
		// fmt.Println(v)
		// panic("")
		c_, _ := EncryptByAes([]byte(v))
		str = strings.Replace(str, v, fmt.Sprintf(`/** Here is hidden scripts %s */`, c_), 1)
	}
	return str
}

func halfDeEct(str string) string {
	ss := regexp.MustCompile(`/\*\* Here is hidden scripts ([\s\S]+?) \*/`).FindAllStringSubmatch(str, -1)
	for _, v := range ss {
		f := v[0]
		c := v[1]
		c_, _ := DecryptByAes(c)
		if c_ != nil {
			str = strings.Replace(str, f, string(c_), 1)
		}
	}
	return str
}

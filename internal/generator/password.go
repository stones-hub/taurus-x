package generator

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
)

// passwordHash 与生成项目 app/helper/crypto.go 的 HashPassword 保持一致：
// SHA-256(password + salt)，hex 编码。init.sql 里的初始管理员散列由此计算，
// 保证生成的 SQL 能被生成的后端直接校验通过。
//
// 注意：这是 admin 底座遗留的单次 sha256 方案（见 docs/安全审计.md #16），
// 第一版不改为 bcrypt；换算法时这里和 helper/crypto.go 必须一起改。
func passwordHash(password, salt string) string {
	sum := sha256.Sum256([]byte(password + salt))
	return hex.EncodeToString(sum[:])
}

// 随机密码字符集：去掉 0/O、1/l/I 这些人眼容易认错的字符。
// 初始密码要由人从终端抄进登录框一次，抄错的代价比少几位熵更高。
const (
	passwordLetters = "abcdefghijkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ"
	passwordDigits  = "23456789"
)

// randomPassword 生成 n 位随机初始管理员密码。
//
// 刻意只用字母和数字、并保证两者都至少出现一次：生成项目的
// helper.ValidatePasswordStrength 要求 6-20 位且字母数字齐全，
// 初始密码必须能过它自己的校验，否则用户第一次改密就会被挡住。
func randomPassword(n int) (string, error) {
	if n < 8 || n > 20 {
		return "", fmt.Errorf("随机密码长度必须在 8-20 之间，收到 %d", n)
	}

	alphabet := passwordLetters + passwordDigits
	buf := make([]byte, n)
	for i := range buf {
		c, err := randomByteFrom(alphabet)
		if err != nil {
			return "", err
		}
		buf[i] = c
	}

	// 全随机有极小概率抽不到数字或字母，所以随机挑两个「不同」的位置兜底。
	// 位置也随机，避免「第一位一定是字母」这种可被利用的固定结构。
	letterPos, err := randomIndex(n)
	if err != nil {
		return "", err
	}
	digitPos, err := randomIndex(n - 1)
	if err != nil {
		return "", err
	}
	if digitPos >= letterPos {
		digitPos++
	}
	if buf[letterPos], err = randomByteFrom(passwordLetters); err != nil {
		return "", err
	}
	if buf[digitPos], err = randomByteFrom(passwordDigits); err != nil {
		return "", err
	}

	return string(buf), nil
}

// randomByteFrom 从字符集里等概率取一个字节。
func randomByteFrom(set string) (byte, error) {
	i, err := randomIndex(len(set))
	if err != nil {
		return 0, err
	}
	return set[i], nil
}

// randomIndex 返回 [0, n) 内的密码学安全随机数。
func randomIndex(n int) (int, error) {
	if n <= 0 {
		return 0, fmt.Errorf("随机范围必须为正数，收到 %d", n)
	}
	v, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		return 0, fmt.Errorf("生成随机数失败: %w", err)
	}
	return int(v.Int64()), nil
}

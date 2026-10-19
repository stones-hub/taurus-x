package generator

import (
	"crypto/sha256"
	"encoding/hex"
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

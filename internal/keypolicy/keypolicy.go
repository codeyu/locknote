// Package keypolicy 检查用户自定义的恢复密钥是否容易被猜到。
//
// 恢复密钥是 16 位 Crockford Base32（约 80 位随机性），能重置主密码、解开全部数据，
// 因此不允许使用明显有规律的密钥。规则需与前端 frontend/src/recoveryKey.ts 保持一致。
// 随机生成的密钥被误判的概率约为万分之五，生成时会自动重试。
package keypolicy

// Alphabet 是恢复密钥使用的 Crockford Base32 字符集（不含 I、L、O、U）。
const Alphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

const (
	minDistinct    = 8
	maxRepeatRun   = 3
	maxSequenceRun = 4
)

// Weakness 描述密钥为什么太弱；空字符串表示没有问题。
type Weakness string

const (
	FewDistinct Weakness = "few_distinct" // 不同字符太少（含重复片段，如 ABCDABCD…）
	RepeatRun   Weakness = "repeat_run"   // 同一字符连续出现 4 次以上
	SequenceRun Weakness = "sequence_run" // 字符表中连续 5 个以上（如 12345、ZYXWV）
)

// Check 检查已归一化（大写、去分隔符）的恢复密钥。
func Check(normalized string) Weakness {
	distinct := map[byte]bool{}
	for i := 0; i < len(normalized); i++ {
		distinct[normalized[i]] = true
	}
	if len(distinct) < minDistinct {
		return FewDistinct
	}

	run := 1
	for i := 1; i < len(normalized); i++ {
		if normalized[i] == normalized[i-1] {
			run++
			if run > maxRepeatRun {
				return RepeatRun
			}
		} else {
			run = 1
		}
	}

	pos := func(b byte) int {
		for i := 0; i < len(Alphabet); i++ {
			if Alphabet[i] == b {
				return i
			}
		}
		return -100
	}
	up, down := 1, 1
	for i := 1; i < len(normalized); i++ {
		diff := pos(normalized[i]) - pos(normalized[i-1])
		if diff == 1 {
			up++
		} else {
			up = 1
		}
		if diff == -1 {
			down++
		} else {
			down = 1
		}
		if up > maxSequenceRun || down > maxSequenceRun {
			return SequenceRun
		}
	}
	return ""
}

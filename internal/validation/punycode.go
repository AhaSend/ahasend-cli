package validation

import "strings"

// The parameters of Punycode, from RFC 3492, section 5.
const (
	punycodeBase        = 36
	punycodeTMin        = 1
	punycodeTMax        = 26
	punycodeSkew        = 38
	punycodeDamp        = 700
	punycodeInitialBias = 72
	punycodeInitialN    = 128
)

// ASCIIDomain returns domain with each label that is not ASCII written as
// "xn--" and its Punycode, as DNS resolves an international domain name,
// such as "xn--bcher-kva.de" for "bücher.de". ASCII labels are kept as they
// are. It does no other mapping or checking: a caller lowercases the domain
// first.
func ASCIIDomain(domain string) string {
	labels := strings.Split(domain, ".")
	for i, label := range labels {
		if !isASCII(label) {
			labels[i] = "xn--" + punycode(label)
		}
	}
	return strings.Join(labels, ".")
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

// punycode encodes label as RFC 3492, section 6.3, describes: its ASCII
// characters, a hyphen when there are any, then the other characters as
// digits.
func punycode(label string) string {
	runes := []rune(label)
	var output strings.Builder
	basic := 0
	for _, r := range runes {
		if r < 0x80 {
			output.WriteRune(r)
			basic++
		}
	}
	if basic > 0 {
		output.WriteByte('-')
	}

	n, delta, bias := rune(punycodeInitialN), 0, punycodeInitialBias
	for handled := basic; handled < len(runes); {
		// The smallest character not handled yet
		next := rune(0x7fffffff)
		for _, r := range runes {
			if r >= n && r < next {
				next = r
			}
		}
		delta += int(next-n) * (handled + 1)
		n = next
		for _, r := range runes {
			if r < n {
				delta++
				continue
			}
			if r > n {
				continue
			}
			q := delta
			for k := punycodeBase; ; k += punycodeBase {
				t := k - bias
				if t < punycodeTMin {
					t = punycodeTMin
				} else if t > punycodeTMax {
					t = punycodeTMax
				}
				if q < t {
					break
				}
				output.WriteByte(punycodeDigit(t + (q-t)%(punycodeBase-t)))
				q = (q - t) / (punycodeBase - t)
			}
			output.WriteByte(punycodeDigit(q))
			bias = punycodeAdapt(delta, handled+1, handled == basic)
			delta = 0
			handled++
		}
		delta++
		n++
	}
	return output.String()
}

// punycodeDigit returns the character of digit, 0 to 35: a to z, then 0 to 9.
func punycodeDigit(digit int) byte {
	if digit < 26 {
		return byte('a' + digit)
	}
	return byte('0' + digit - 26)
}

// punycodeAdapt is the bias adaptation of RFC 3492, section 6.1.
func punycodeAdapt(delta, points int, first bool) int {
	if first {
		delta /= punycodeDamp
	} else {
		delta /= 2
	}
	delta += delta / points
	k := 0
	for delta > ((punycodeBase-punycodeTMin)*punycodeTMax)/2 {
		delta /= punycodeBase - punycodeTMin
		k += punycodeBase
	}
	return k + (punycodeBase-punycodeTMin+1)*delta/(delta+punycodeSkew)
}

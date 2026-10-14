package validation

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestASCIIDomainWritesInternationalLabelsInPunycode(t *testing.T) {
	tests := map[string]string{
		"example.com":    "example.com",
		"bücher.de":      "xn--bcher-kva.de",
		"www.münchen.de": "www.xn--mnchen-3ya.de",
		// Samples of RFC 3492, section 7.1
		"他们为什么不说中文.com":         "xn--ihqwcrb4cv8a8dqg056pqjye.com",
		"ليهمابتكلموشعربي؟.com": "xn--egbpdaj6bu4bxfgehfvwxn.com",
		"3年b組金八先生.jp":           "xn--3b-ww4c5e180e575a65lsy2b.jp",
		"例え.テスト":                "xn--r8jz45g.xn--zckzah",
	}
	for domain, want := range tests {
		assert.Equal(t, want, ASCIIDomain(domain), domain)
	}
}

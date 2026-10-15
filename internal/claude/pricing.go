package claude

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/licoy/quosparr/internal/usage/decimal"
)

// PricingVersion is the built-in Claude price table. Rates were checked
// against https://platform.claude.com/docs/en/about-claude/pricing on
// 2026-10-14. Only Sonnet 4 and 4.5 keep the 1M-context premium (2x input,
// 1.5x output above 200K prompt tokens); newer models have none.
// Amounts are US dollars per million tokens. Unknown models are not priced.
const PricingVersion = "anthropic-builtin-2026-10-14"

// haiku55PromptThreshold is the prompt size, in tokens, at which
// claude-haiku-5-5 switches rate. The prompt is uncached input plus both
// cache writes plus cache reads. The higher rate applies only above this
// count, not at it.
const haiku55PromptThreshold int64 = 100_000

// sonnet4LongPromptThreshold is the Sonnet 4 / 4.5 long-context threshold,
// counted the same way.
const sonnet4LongPromptThreshold int64 = 200_000

// TokenParts is one deduped Claude Code model call.
type TokenParts struct {
	Input        int64
	CacheWrite5m int64
	CacheWrite1h int64
	CacheRead    int64
	Output       int64
	Fast         bool
}

// PromptTokens is the prompt size the tiered rates compare.
func (p TokenParts) PromptTokens() int64 {
	return p.Input + p.CacheWrite5m + p.CacheWrite1h + p.CacheRead
}

type rate struct {
	in, c5, c1, cr, out decimal.Decimal
}

type modelPrice struct {
	std         rate
	fast        *rate
	fastAsStd   bool
	above       *rate
	threshold   int64
	usSurcharge bool
}

func priced(in, c5, c1, cr, out string) rate {
	return rate{
		in:  decimal.MustParse(in),
		c5:  decimal.MustParse(c5),
		c1:  decimal.MustParse(c1),
		cr:  decimal.MustParse(cr),
		out: decimal.MustParse(out),
	}
}

func fastRate(in, out, readFactor string) rate {
	input := decimal.MustParse(in)
	return rate{
		in:  input,
		c5:  input.Mul(decimal.MustParse("1.25")),
		c1:  input.Mul(decimal.MustParse("2")),
		cr:  input.Mul(decimal.MustParse(readFactor)),
		out: decimal.MustParse(out),
	}
}

var usSurcharge = decimal.MustParse("1.1")

func withUS(p modelPrice) modelPrice {
	p.usSurcharge = true
	return p
}

func withFast(p modelPrice, fast rate) modelPrice {
	p.fast = &fast
	return p
}

func withFastStandard(p modelPrice) modelPrice {
	p.fastAsStd = true
	return p
}

func withAbove(p modelPrice, above rate, threshold int64) modelPrice {
	p.above = &above
	p.threshold = threshold
	return p
}

// pricingTable is keyed by normalized model id.
// Fast cache rates for opus 5.5 are 1.25, 2 and 0.05 times the fast input.
// opus 5 and opus 4.8 use the published fast cache rates 12.5 / 20 / 1.
// opus 4.7 fast is not priced. opus 4.6 fast uses the standard rates.
var pricingTable = map[string]modelPrice{
	"claude-fable-5-1":  withUS(modelPrice{std: priced("10", "12.5", "20", "0.25", "50")}),
	"claude-mythos-5-1": withUS(modelPrice{std: priced("10", "12.5", "20", "0.25", "50")}),
	"claude-fable-5":    withUS(modelPrice{std: priced("10", "12.5", "20", "1", "50")}),
	"claude-mythos-5":   withUS(modelPrice{std: priced("10", "12.5", "20", "1", "50")}),
	"claude-opus-5-5":   withUS(withFast(modelPrice{std: priced("4", "5", "8", "0.20", "20")}, fastRate("8", "40", "0.05"))),
	"claude-opus-5":     withUS(withFast(modelPrice{std: priced("5", "6.25", "10", "0.50", "25")}, priced("10", "12.5", "20", "1", "50"))),
	"claude-opus-4-8":   withUS(withFast(modelPrice{std: priced("5", "6.25", "10", "0.50", "25")}, priced("10", "12.5", "20", "1", "50"))),
	"claude-opus-4-7":   withUS(modelPrice{std: priced("5", "6.25", "10", "0.50", "25")}),
	"claude-opus-4-6":   withUS(withFastStandard(modelPrice{std: priced("5", "6.25", "10", "0.50", "25")})),
	"claude-opus-4-5":   modelPrice{std: priced("5", "6.25", "10", "0.50", "25")},
	"claude-opus-4-1":   modelPrice{std: priced("15", "18.75", "30", "1.50", "75")},
	"claude-opus-4-0":   modelPrice{std: priced("15", "18.75", "30", "1.50", "75")},
	"claude-opus-4":     modelPrice{std: priced("15", "18.75", "30", "1.50", "75")},
	"claude-sonnet-5-5": withUS(modelPrice{std: priced("2", "2.5", "4", "0.10", "10")}),
	"claude-sonnet-5":   withUS(modelPrice{std: priced("2", "2.5", "4", "0.20", "10")}),
	"claude-sonnet-4-6": withUS(modelPrice{std: priced("3", "3.75", "6", "0.30", "15")}),
	"claude-sonnet-4-5": sonnet4Price,
	"claude-sonnet-4-0": sonnet4Price,
	"claude-sonnet-4":   sonnet4Price,
	"claude-3-7-sonnet": modelPrice{std: priced("3", "3.75", "6", "0.30", "15")},
	"claude-haiku-4-5":  modelPrice{std: priced("1", "1.25", "2", "0.10", "5")},
	"claude-haiku-3-5":  modelPrice{std: priced("0.8", "1", "1.6", "0.08", "4")},
	"claude-3-5-haiku":  modelPrice{std: priced("0.8", "1", "1.6", "0.08", "4")},
	"claude-haiku-5-5": withUS(withAbove(
		modelPrice{std: priced("0.10", "0.125", "0.20", "0.01", "0.50")},
		priced("0.50", "0.625", "1", "0.05", "2.50"),
		haiku55PromptThreshold,
	)),
}

var sonnet4Price = withAbove(
	modelPrice{std: priced("3", "3.75", "6", "0.30", "15")},
	priced("6", "7.5", "12", "0.60", "22.5"),
	sonnet4LongPromptThreshold,
)

var (
	modelBracket = regexp.MustCompile(`\[[^\[\]]*\]$`)
	modelVertex  = regexp.MustCompile(`@(?:\d{8}|\d{4}-\d{2}-\d{2})$`)
	modelDate    = regexp.MustCompile(`-\d{8}$`)
	modelVendor  = regexp.MustCompile(`-v\d+:\d+$`)
)

// NormalizeModel lower-cases a model id and removes whitespace, a trailing
// bracket suffix, a Vertex @date, a Bedrock region prefix, an anthropic.
// prefix, a -v1:0 style suffix and a -YYYYMMDD suffix.
func NormalizeModel(model string) string {
	s := strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, strings.ToLower(model))
	s = modelBracket.ReplaceAllString(s, "")
	s = modelVertex.ReplaceAllString(s, "")
	s = stripRegionPrefix(s)
	s = strings.TrimPrefix(s, "anthropic.")
	for {
		next := modelVendor.ReplaceAllString(modelDate.ReplaceAllString(s, ""), "")
		if next == s {
			break
		}
		s = next
	}
	return s
}

func stripRegionPrefix(s string) string {
	for _, prefix := range []string{"us.", "eu.", "apac.", "global."} {
		if strings.HasPrefix(s, prefix) {
			return strings.TrimPrefix(s, prefix)
		}
	}
	return s
}

// PricingCost prices one call. ok is false for an unknown model and for
// speed=fast when that model has no fast rate (opus 4.7 included). inference
// geo "us" multiplies every rate by 1.1 for 4.6-and-newer models, including
// haiku 5.5, sonnet 5, opus 5, fable and mythos.
func PricingCost(model string, parts TokenParts, geo string) (decimal.Decimal, bool) {
	price, ok := pricingTable[NormalizeModel(model)]
	if !ok {
		return decimal.Decimal{}, false
	}
	chosen, ok := chooseRate(price, parts)
	if !ok {
		return decimal.Decimal{}, false
	}
	if price.usSurcharge && strings.EqualFold(strings.TrimSpace(geo), "us") {
		chosen = scaleRate(chosen, usSurcharge)
	}
	return costFor(chosen, parts), true
}

func chooseRate(price modelPrice, parts TokenParts) (rate, bool) {
	if parts.Fast {
		if price.fastAsStd {
			return price.std, true
		}
		if price.fast == nil {
			return rate{}, false
		}
		return *price.fast, true
	}
	if price.above != nil && parts.PromptTokens() > price.threshold {
		return *price.above, true
	}
	return price.std, true
}

func scaleRate(r rate, factor decimal.Decimal) rate {
	return rate{
		in: r.in.Mul(factor), c5: r.c5.Mul(factor), c1: r.c1.Mul(factor),
		cr: r.cr.Mul(factor), out: r.out.Mul(factor),
	}
}

func costFor(r rate, parts TokenParts) decimal.Decimal {
	million := decimal.FromInt(1_000_000)
	part := func(tokens int64, rate decimal.Decimal) decimal.Decimal {
		if tokens <= 0 {
			return decimal.Decimal{}
		}
		return decimal.FromInt64(tokens).Div(million).Mul(rate)
	}
	return part(parts.Input, r.in).
		Add(part(parts.CacheWrite5m, r.c5)).
		Add(part(parts.CacheWrite1h, r.c1)).
		Add(part(parts.CacheRead, r.cr)).
		Add(part(parts.Output, r.out))
}

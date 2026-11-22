package hotkeystream

import "math"

const (
	layoutBucketMinutes = 3
	layoutBuckets       = 5
)

var layoutCategories = []string{CategoryHall, CategoryProd, CategoryTech, CategoryComsat, CategoryUnits}

// LayoutVector flattens a signature into what each key held in each early game phase, as shares of all
// presses. Late minutes fold into the last phase.
func LayoutVector(sig *Signature) []float64 {
	v := make([]float64, 10*layoutBuckets*len(layoutCategories))
	if sig == nil {
		return v
	}
	for _, key := range sig.Keys {
		if key.Key < 0 || key.Key > 9 {
			continue
		}
		for _, run := range key.Runs {
			cat := -1
			base, _ := splitCategory(run.Category)
			for i, c := range layoutCategories {
				if c == base {
					cat = i
				}
			}
			minutes := run.EndMin - run.StartMin + 1
			if cat < 0 || minutes <= 0 {
				continue
			}
			perMinute := float64(run.Presses) / float64(minutes)
			for m := run.StartMin; m <= run.EndMin; m++ {
				bucket := min(m/layoutBucketMinutes, layoutBuckets-1)
				v[(key.Key*layoutBuckets+bucket)*len(layoutCategories)+cat] += perMinute
			}
		}
	}
	return v
}

// LayoutSimilarity is the cosine between two signatures' layout vectors: 1 for the same keys holding the
// same things at the same time, 0 for nothing in common.
func LayoutSimilarity(a, b *Signature) float64 {
	va, vb := LayoutVector(a), LayoutVector(b)
	var dot, na, nb float64
	for i := range va {
		dot += va[i] * vb[i]
		na += va[i] * va[i]
		nb += vb[i] * vb[i]
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / math.Sqrt(na*nb)
}

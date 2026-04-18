package dashboard

import (
	"io/fs"
	"math"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode"
)

const workflowMapVisualMinScore = 0.56

type workflowMapImageCandidate struct {
	FileName   string
	PrettyName string
}

var (
	workflowMapImagesOnce sync.Once
	workflowMapImages     []workflowMapImageCandidate
	workflowMapImagesErr  error
)

func (d *Dashboard) resolveWorkflowMapVisual(mapName string) workflowMapVisual {
	out := workflowMapVisual{
		RequestedMap: strings.TrimSpace(mapName),
	}
	if out.RequestedMap == "" {
		out.ResolutionNote = "missing replay map name"
		return out
	}

	candidates, err := workflowMapImageCandidates()
	if err != nil {
		out.ResolutionNote = "map image catalog unavailable"
		return out
	}
	if len(candidates) == 0 {
		out.ResolutionNote = "no map images are bundled"
		return out
	}

	best := workflowMapImageCandidate{}
	bestScore := 0.0
	for _, candidate := range candidates {
		score := scoreWorkflowMapName(out.RequestedMap, candidate.PrettyName)
		if score > bestScore {
			bestScore = score
			best = candidate
		}
	}
	if bestScore < workflowMapVisualMinScore {
		out.ResolutionNote = "no confident map-image match found"
		return out
	}

	url := "/map-images/" + best.FileName
	out.Available = true
	out.URL = url
	out.ThumbnailURL = url
	out.MatchedImage = best.FileName
	out.MatchedScore = roundToDecimals(bestScore, 4)
	return out
}

func workflowMapImageCandidates() ([]workflowMapImageCandidate, error) {
	workflowMapImagesOnce.Do(func() {
		paths, err := fs.Glob(embeddedFrontendBuild, "frontend/build/map-images/*")
		if err != nil {
			workflowMapImagesErr = err
			return
		}
		items := make([]workflowMapImageCandidate, 0, len(paths))
		for _, p := range paths {
			fileName := filepath.Base(p)
			ext := strings.ToLower(filepath.Ext(fileName))
			if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".webp" {
				continue
			}
			base := strings.TrimSuffix(fileName, filepath.Ext(fileName))
			items = append(items, workflowMapImageCandidate{
				FileName:   fileName,
				PrettyName: base,
			})
		}
		sort.Slice(items, func(i, j int) bool {
			return items[i].FileName < items[j].FileName
		})
		workflowMapImages = items
	})
	if workflowMapImagesErr != nil {
		return nil, workflowMapImagesErr
	}
	out := make([]workflowMapImageCandidate, len(workflowMapImages))
	copy(out, workflowMapImages)
	return out, nil
}

func scoreWorkflowMapName(query string, candidate string) float64 {
	q := normalizeWorkflowMapName(query)
	c := normalizeWorkflowMapName(candidate)
	if q == "" || c == "" {
		return 0
	}
	if q == c {
		return 1
	}

	qCompact := strings.ReplaceAll(q, " ", "")
	cCompact := strings.ReplaceAll(c, " ", "")
	if qCompact == cCompact {
		return 0.99
	}
	if qCompact != "" && cCompact != "" && (strings.Contains(qCompact, cCompact) || strings.Contains(cCompact, qCompact)) {
		return 0.94
	}

	edit := 1 - (float64(workflowLevenshtein(qCompact, cCompact)) / float64(workflowMaxInt(len([]rune(qCompact)), len([]rune(cCompact)))))
	token := workflowTokenSimilarity(q, c)
	prefix := 0.0
	if strings.HasPrefix(qCompact, cCompact) || strings.HasPrefix(cCompact, qCompact) {
		prefix = 1
	}
	return 0.5*edit + 0.35*token + 0.15*prefix
}

func normalizeWorkflowMapName(s string) string {
	s = strings.ToLower(s)
	s = workflowStripControl(s)
	replacer := strings.NewReplacer(
		"_", " ",
		"-", " ",
		"(", "",
		")", "",
		"[", "",
		"]", "",
		"{", "",
		"}", "",
		".", "",
		",", "",
		"'", "",
		"\"", "",
		":", "",
		";", "",
		"/", "",
		"\\", "",
		"&", " and ",
	)
	s = replacer.Replace(s)
	tokens := strings.Fields(s)
	filtered := make([]string, 0, len(tokens))
	for _, token := range tokens {
		token = workflowKeepAlphaNum(token)
		if token == "" {
			continue
		}
		if workflowIsVersionToken(token) {
			continue
		}
		filtered = append(filtered, token)
	}
	return strings.Join(filtered, " ")
}

func workflowTokenSimilarity(a string, b string) float64 {
	aSet := workflowTokenSet(a)
	bSet := workflowTokenSet(b)
	if len(aSet) == 0 || len(bSet) == 0 {
		return 0
	}
	common := 0
	for token := range aSet {
		if bSet[token] {
			common++
		}
	}
	return float64(common) / float64(len(aSet)+len(bSet)-common)
}

func workflowTokenSet(s string) map[string]bool {
	out := map[string]bool{}
	for _, token := range strings.Fields(s) {
		if token == "" {
			continue
		}
		out[token] = true
	}
	return out
}

func workflowLevenshtein(a string, b string) int {
	ra := []rune(a)
	rb := []rune(b)
	if len(ra) == 0 {
		return len(rb)
	}
	if len(rb) == 0 {
		return len(ra)
	}
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 0
			if ra[i-1] != rb[j-1] {
				cost = 1
			}
			cur[j] = workflowMin3(cur[j-1]+1, prev[j]+1, prev[j-1]+cost)
		}
		copy(prev, cur)
	}
	return prev[len(rb)]
}

func workflowStripControl(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsControl(r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func workflowKeepAlphaNum(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsDigit(r) || unicode.IsLetter(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func workflowIsVersionToken(token string) bool {
	token = strings.TrimPrefix(token, "v")
	if strings.Count(token, ".") > 3 {
		return false
	}
	parts := strings.Split(token, ".")
	if len(parts) == 0 {
		return false
	}
	for _, part := range parts {
		if part == "" {
			return false
		}
		if _, err := strconv.Atoi(part); err != nil {
			return false
		}
	}
	return true
}

func workflowMaxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

func workflowMin3(a int, b int, c int) int {
	if a <= b && a <= c {
		return a
	}
	if b <= c {
		return b
	}
	return c
}

func roundToDecimals(value float64, decimals int) float64 {
	mult := math.Pow(10, float64(decimals))
	return math.Round(value*mult) / mult
}

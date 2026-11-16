package main

import (
	"errors"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

const minConfirmedAccounts = 30

var errFormatChanged = errors.New("the TL post format changed; update scripts/sb-pros parser against testdata snapshot")

type tlAccount struct {
	ID       uint32
	Name     string
	Formerly []tlAccount
}

type tlEntry struct {
	Label    string
	Race     string
	Accounts []tlAccount
}

var (
	linkRe     = regexp.MustCompile(`(?is)<a\s[^>]*href="([^"]*)"[^>]*>(.*?)</a>`)
	brRe       = regexp.MustCompile(`(?i)<br\s*/?>`)
	tagRe      = regexp.MustCompile(`(?s)<[^>]*>`)
	headingRe  = regexp.MustCompile(`(?i)^[IVX]+\.\s*(TERRAN|ZERG|PROTOSS)$`)
	boldRe     = regexp.MustCompile(`(?is)^\s*<b>(.*?)</b>`)
	updateRe   = regexp.MustCompile(`(?i)^UPDATE\s*\d+\s*:`)
	uncertainW = []string{"potentially", "possibly", "maybe", "probably"}
)

func formatChanged(format string, args ...any) error {
	return fmt.Errorf("%w: %s", errFormatChanged, fmt.Sprintf(format, args...))
}

func firstPost(page string) (string, error) {
	i := strings.Index(page, `class="forumPost"`)
	if i < 0 {
		return "", formatChanged(`no class="forumPost" element`)
	}
	rest := page[i:]
	start := strings.Index(rest, "<section>")
	if start < 0 {
		return "", formatChanged("first post has no <section>")
	}
	rest = rest[start+len("<section>"):]
	end := strings.Index(rest, "</section>")
	if end < 0 {
		return "", formatChanged("first post <section> is not closed")
	}
	return rest[:end], nil
}

func plainText(s string) string {
	return strings.TrimSpace(html.UnescapeString(tagRe.ReplaceAllString(s, "")))
}

// parsePage extracts the confirmed accounts from the first post of the TL thread.
func parsePage(page string) ([]tlEntry, error) {
	post, err := firstPost(page)
	if err != nil {
		return nil, err
	}
	var (
		entries   []tlEntry
		race      string
		headings  int
		confirmed int
	)
	for _, line := range brRe.Split(post, -1) {
		if m := boldRe.FindStringSubmatch(line); m != nil {
			if h := headingRe.FindStringSubmatch(plainText(m[1])); h != nil {
				race = strings.ToLower(h[1])
				headings++
				continue
			}
			if race != "" {
				break
			}
		}
		if updateRe.MatchString(plainText(line)) {
			break
		}
		if race == "" || !linkRe.MatchString(line) {
			continue
		}
		entry, err := parseLine(line)
		if err != nil {
			return nil, err
		}
		if len(entry.Accounts) == 0 {
			continue
		}
		entry.Race = race
		confirmed += len(entry.Accounts)
		entries = append(entries, entry)
	}
	if headings == 0 {
		return nil, formatChanged("no race headings (I. TERRAN, II. ZERG, III. PROTOSS) found")
	}
	if confirmed < minConfirmedAccounts {
		return nil, formatChanged("only %d confirmed accounts parsed, expected at least %d", confirmed, minConfirmedAccounts)
	}
	return entries, nil
}

func parseLine(line string) (tlEntry, error) {
	links := linkRe.FindAllStringSubmatchIndex(line, -1)
	colon := strings.Index(line[:links[0][0]], ":")
	if colon < 0 {
		return tlEntry{}, formatChanged("line has no label before its first link: %q", plainText(line))
	}
	entry := tlEntry{Label: plainText(line[:colon])}
	if entry.Label == "" {
		return tlEntry{}, formatChanged("line has an empty label: %q", plainText(line))
	}
	var inParen, parenFormerly bool
	prev := colon + 1
	for _, l := range links {
		between := plainText(line[prev:l[0]])
		lower := strings.ToLower(between)
		for i, r := range lower {
			switch r {
			case '(':
				inParen = true
				parenFormerly = strings.HasPrefix(strings.TrimSpace(lower[i+1:]), "formerly")
			case ')':
				inParen, parenFormerly = false, false
			}
		}
		prev = l[1]
		acc, err := parseLink(line[l[2]:l[3]], line[l[4]:l[5]])
		if err != nil {
			return tlEntry{}, err
		}
		switch {
		case inParen && parenFormerly:
			if len(entry.Accounts) == 0 {
				return tlEntry{}, formatChanged("%s: former account listed before any current one", entry.Label)
			}
			last := &entry.Accounts[len(entry.Accounts)-1]
			last.Formerly = append(last.Formerly, acc)
		case inParen || uncertain(lower):
		default:
			entry.Accounts = append(entry.Accounts, acc)
		}
	}
	return entry, nil
}

func uncertain(s string) bool {
	for _, w := range uncertainW {
		if strings.Contains(s, w) {
			return true
		}
	}
	return false
}

func parseLink(href, text string) (tlAccount, error) {
	u, err := url.Parse(html.UnescapeString(href))
	if err != nil {
		return tlAccount{}, formatChanged("unparseable link %q", href)
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 || parts[0] != "users" || !strings.HasSuffix(u.Hostname(), "shieldbattery.net") {
		return tlAccount{}, formatChanged("link %q is not a ShieldBattery user link", href)
	}
	id, err := strconv.ParseUint(parts[1], 10, 32)
	if err != nil || id == 0 {
		return tlAccount{}, formatChanged("non-numeric ShieldBattery user id %q in %q", parts[1], href)
	}
	name := plainText(text)
	if len(parts) >= 3 && parts[2] != "" {
		name = parts[2]
	}
	return tlAccount{ID: uint32(id), Name: name}, nil
}

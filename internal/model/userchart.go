package model

import (
	"sort"
	"strings"
	"time"
)

// ChartTopUsers is how many distinct contributors (plus optional "others") we stack per column.
const ChartTopUsers = 6

// UserChartSeries is a stacked time-axis chart with one segment per contributor (plus "others").
type UserChartSeries struct {
	Users    []string           `json:"users"`
	Buckets  []UserChartBucket  `json:"buckets"`
	MaxTotal int                `json:"max_total"`
}

// UserChartBucket holds per-user counts for one time bucket; Counts align with UserChartSeries.Users.
type UserChartBucket struct {
	StartRFC3339 string `json:"start"`
	Counts       []int  `json:"counts"`
}

// UserLabel groups commits by author display name, falling back to email local-part.
func UserLabel(name, email string) string {
	s := strings.TrimSpace(name)
	if s != "" {
		return s
	}
	s = strings.TrimSpace(email)
	if i := strings.IndexByte(s, '@'); i > 0 {
		return s[:i]
	}
	if s != "" {
		return s
	}
	return "unknown"
}

// IssueChartActor attributes closed-issue activity to assignee when present, else author.
func IssueChartActor(is IssueRow) string {
	if s := strings.TrimSpace(is.AssigneeName); s != "" {
		return s
	}
	if s := strings.TrimSpace(is.AuthorName); s != "" {
		return s
	}
	return "unknown"
}

// BuildUserChartFromAgg turns raw bucket→user counts into a top-N stacked series.
func BuildUserChartFromAgg(since, until time.Time, tr TimeRange, perBucket map[int64]map[string]int, topN int) *UserChartSeries {
	starts := EnumerateBucketStarts(since, until, tr)
	if len(starts) == 0 {
		return &UserChartSeries{MaxTotal: 1}
	}
	if topN <= 0 {
		topN = ChartTopUsers
	}

	n := len(starts)
	type bucketRow struct {
		start time.Time
		m     map[string]int
	}
	rows := make([]bucketRow, n)
	totals := make(map[string]int)
	for i, st := range starts {
		k := st.Unix()
		m := perBucket[k]
		if m == nil {
			m = map[string]int{}
		}
		cp := make(map[string]int, len(m))
		for u, c := range m {
			cp[u] = c
			totals[u] += c
		}
		rows[i] = bucketRow{start: st, m: cp}
	}

	type pair struct {
		u string
		t int
	}
	var pairs []pair
	for u, t := range totals {
		pairs = append(pairs, pair{u, t})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].t != pairs[j].t {
			return pairs[i].t > pairs[j].t
		}
		return pairs[i].u < pairs[j].u
	})

	main := make([]string, 0, topN+1)
	rest := make(map[string]struct{})
	for i, p := range pairs {
		if i < topN {
			main = append(main, p.u)
		} else {
			rest[p.u] = struct{}{}
		}
	}
	if len(rest) > 0 {
		main = append(main, "others")
	}
	if len(main) == 0 {
		return &UserChartSeries{
			Users:    nil,
			Buckets:  nil,
			MaxTotal: 1,
		}
	}

	outBuckets := make([]UserChartBucket, n)
	maxTot := 1
	for i, row := range rows {
		counts := make([]int, len(main))
		for j, uname := range main {
			if uname == "others" {
				sum := 0
				for u := range rest {
					sum += row.m[u]
				}
				counts[j] = sum
				continue
			}
			counts[j] = row.m[uname]
		}
		sum := 0
		for _, v := range counts {
			sum += v
		}
		if sum > maxTot {
			maxTot = sum
		}
		outBuckets[i] = UserChartBucket{
			StartRFC3339: row.start.Format(time.RFC3339),
			Counts:       counts,
		}
	}

	return &UserChartSeries{
		Users:    main,
		Buckets:  outBuckets,
		MaxTotal: maxTot,
	}
}
